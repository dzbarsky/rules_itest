package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"rules_itest/logger"
	"rules_itest/runner"
	"rules_itest/svclib"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	flags := flag.NewFlagSet("svcinit", flag.ExitOnError)

	serviceSpecsPath := flags.String("svc.specs-path", "", "File defining which services to run")
	allowSvcctl := flags.Bool("svc.allow-svcctl", false, "If true, spawns a server to handle svcctl commands")
	_ = allowSvcctl

	shouldHotReload := os.Getenv("IBAZEL_NOTIFY_CHANGES") == "y"
	testLabel := os.Getenv("TEST_TARGET")

	interactiveCh := make(chan struct{}, 100)
	if shouldHotReload {
		go func() {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				fmt.Println(scanner.Text())

				// TODO: better notification setup needed
				interactiveCh <- struct{}{}
				//close(interactiveCh)
				//interactiveCh = make(chan struct{})
			}
		}()
	}

	// the flags library doesn't have a good way to ignore unknown args and return them
	// so we do a hacky thing to achieve that behavior here.
	// only support -flag=value and -flag style flags for svcinit (-flag value is *not* supported)
	// everything else is passed to the test runner.
	// TODO: is this to support --test_arg? Do we need it?
	isSvcInitFlag := func(flagName string) bool {
		return flagName == "help" || flagName == "h" || flags.Lookup(flagName) != nil
	}
	var svcInitArgs []string
	var testArgs []string
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--" {
			testArgs = append(testArgs, os.Args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") {
			// not a flag, just assume this is a test args
			testArgs = append(testArgs, arg)
			continue
		}

		flagName := strings.TrimLeft(strings.Split(arg, "=")[0], "-")
		if isSvcInitFlag(flagName) {
			svcInitArgs = append(svcInitArgs, arg)
		} else {
			testArgs = append(testArgs, arg)
		}
	}
	_ = flags.Parse(svcInitArgs)

	// Sockets have a short max path length (108 chars) so the TEST_TMPDIR path is way too long.
	// Put them in the OS temp location - note that this is per-test (i.e. hermetic) on linux anyway.
	socketDir, err := os.MkdirTemp("", "")
	must(err)
	os.Setenv("SOCKET_DIR", socketDir)
	defer os.RemoveAll(socketDir)

	// If we are under `bazel run` for a service group, we may not have TEST_TMPDIR set.
	tmpDir := os.Getenv("TEST_TMPDIR")
	if tmpDir == "" {
		var err error
		tmpDir, err = os.MkdirTemp("", strings.ReplaceAll(testLabel, "/", "_"))
		must(err)
		defer os.RemoveAll(tmpDir)
	}
	os.Setenv("TMPDIR", tmpDir)

	isOneShot := !shouldHotReload && testLabel != ""

	serviceSpecs, err := readVersionedServiceSpecs(*serviceSpecsPath)
	must(err)

	/*if *allowSvcctl {
		addr := net.Listen(network, address)
	}*/

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	r := runner.New(ctx, serviceSpecs)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		count := 0
		for range signalCh {
			if count == 0 {
				log.Println("Shutdown requested, exiting gracefully. Press Ctrl-C again to force exit")
				cancelFunc()
				count++
			} else {
				log.Println("Multiple Ctrl-C detected, force-exiting")
				os.Exit(1)
			}
		}
	}()

	criticalPath, err := r.StartAll()
	if errors.Is(err, context.Canceled) {
		_, err := r.StopAll()
		must(err)
		return
	}
	must(err)

	// API is                 NewWriter(output io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) *Writer
	reportWriter := tabwriter.NewWriter(os.Stdout, 0, 8, 8, ' ', 0)
	buf := bytes.NewBuffer(nil)

	for {
		buf.WriteString("\nTarget\tCritical Path Contribution\n")
		for _, task := range criticalPath {
			buf.WriteString(fmt.Sprintf("%s\t%s\n", task.Key(), task.Duration()))
		}
		_, err := reportWriter.Write(buf.Bytes())
		must(err)
		buf.Reset()
		err = reportWriter.Flush()
		must(err)

		var testCmd *exec.Cmd
		var testErr error
		if testLabel != "" {
			fmt.Println("")
			log.Printf("Executing test: %s\n", strings.Join(testArgs, " "))
			testCmd = exec.CommandContext(ctx, testArgs[0], testArgs[1:]...)
			testCmd.Stdout = os.Stdout
			testCmd.Stderr = os.Stderr

			testStartTime := time.Now()

			if err := testCmd.Start(); err != nil {
				panic(err)
			}

			testErr = testCmd.Wait()

			testDuration := time.Since(testStartTime)
			log.Printf("Test duration: %s\n", testDuration)
		}

		fmt.Println()

		if isOneShot {
			buf.WriteString("Target\tUser Time\tSystem Time\n")
			states, err := r.StopAll()
			must(err)
			for label, state := range states {
				buf.WriteString(fmt.Sprintf("%s\t%s\t%s\n",
					label, state.UserTime(), state.SystemTime()))
			}
		} else {
			buf.WriteString("Target\tStartup Time\n")
			durations := r.GetStartDurations()
			for label, duration := range durations {
				buf.WriteString(fmt.Sprintf("%s\t%s\n", label, duration))
			}
		}

		if testLabel != "" {
			buf.WriteString(fmt.Sprintf("%s\t%s\t%s\n",
				testLabel, testCmd.ProcessState.UserTime(), testCmd.ProcessState.SystemTime()))
		}
		buf.WriteRune('\n')
		_, err = reportWriter.Write(buf.Bytes())
		must(err)
		buf.Reset()
		err = reportWriter.Flush()
		must(err)

		if testErr != nil {
			log.Printf("Encountered error during test run: %s\n", testErr)
			if isOneShot {
				os.Exit(1)
			}
		}

		if isOneShot {
			break
		}

		select {
		case <-ctx.Done():
			log.Println("Shutting down services.")
			_, err := r.StopAll()
			must(err)
			log.Println("Cleaning up.")
			return
		case <-interactiveCh:
		}

		// Restart any services as needed.
		serviceSpecs, err := readVersionedServiceSpecs(*serviceSpecsPath)
		must(err)

		criticalPath, err = r.UpdateSpecsAndRestart(serviceSpecs)
		must(err)
	}
}

func readVersionedServiceSpecs(
	path string,
) (
	map[string]svclib.VersionedServiceSpec, error,
) {
	data, err := os.ReadFile(path)
	must(err)

	var serviceSpecs map[string]svclib.ServiceSpec
	err = json.Unmarshal(data, &serviceSpecs)
	must(err)

	tmpDir := os.Getenv("TMPDIR")
	socketDir := os.Getenv("SOCKET_DIR")

	ports := svclib.Ports{}
	versionedServiceSpecs := make(map[string]svclib.VersionedServiceSpec, len(serviceSpecs))
	for label, serviceSpec := range serviceSpecs {
		version, err := os.ReadFile(serviceSpec.VersionFile)
		if err != nil {
			return nil, err
		}

		s := svclib.VersionedServiceSpec{
			ServiceSpec: serviceSpec,
			Version:     string(version),
		}

		s.Color = logger.Colorize(s.Label)

		// Note, this can cause collisions. So be careful!
		if s.ServiceSpec.AutoassignPort {

			// We do a bit of a dance here to set SO_LINGER to 0. For details, see
			// https://stackoverflow.com/questions/71975992/what-really-is-the-linger-time-that-can-be-set-with-so-linger-on-sockets
			lc := net.ListenConfig{
				Control: func(network, address string, conn syscall.RawConn) error {
					var setSockoptErr error
					err := conn.Control(func(fd uintptr) {
						setSockoptErr = syscall.SetsockoptLinger(int(fd), syscall.SOL_SOCKET, syscall.SO_LINGER, &syscall.Linger{
							Onoff:  1,
							Linger: 0,
						})
					})
					if err != nil {
						return err
					}
					return setSockoptErr
				},
			}

			listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
			if err != nil {
				return nil, err
			}
			_, port, err := net.SplitHostPort(listener.Addr().String())
			if err != nil {
				return nil, err
			}
			err = listener.Close()
			if err != nil {
				return nil, err
			}

			fmt.Printf("Assigning port %s to %s\n", port, s.Label)

			s.AssignedPort = port
			ports.Set(s.Label, port)

			for i := range s.ServiceSpec.Args {
				s.Args[i] = strings.ReplaceAll(s.Args[i], "$${PORT}", port)
			}
			s.HttpHealthCheckAddress = strings.ReplaceAll(s.HttpHealthCheckAddress, "$${PORT}", port)
		}

		for i := range s.Args {
			s.Args[i] = strings.ReplaceAll(s.Args[i], "$${TMPDIR}", tmpDir)
			s.Args[i] = strings.ReplaceAll(s.Args[i], "$${SOCKET_DIR}", socketDir)
		}

		versionedServiceSpecs[label] = s
	}

	serializedPorts, err := ports.Marshal()
	must(err)
	os.Setenv("ASSIGNED_PORTS", string(serializedPorts))

	return versionedServiceSpecs, nil
}
