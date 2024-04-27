package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

	"github.com/bazelbuild/rules_go/go/runfiles"

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
	serviceSpecsPath, err := runfiles.Rlocation(os.Getenv("SVCINIT_SERVICE_SPECS_RLOCATION_PATH"))
	must(err)

	enablePerServiceReload := os.Getenv("SVCINIT_ENABLE_PER_SERVICE_RELOAD") == "True"
	shouldHotReload := os.Getenv("IBAZEL_NOTIFY_CHANGES") == "y"
	testLabel := os.Getenv("TEST_TARGET")

	interactiveCh := make(chan string, 100)
	if shouldHotReload {
		go func() {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				// TODO: better notification setup needed
				interactiveCh <- scanner.Text()
				//close(interactiveCh)
				//interactiveCh = make(chan struct{})
			}
		}()
	}

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
	os.Setenv("TEST_TMPDIR", tmpDir)
	os.Setenv("TMPDIR", tmpDir)

	getAssignedPortBinPath, err := runfiles.Rlocation(os.Getenv("SVCINIT_GET_ASSIGNED_PORT_BIN_RLOCATION_PATH"))
	must(err)
	os.Setenv("GET_ASSIGNED_PORT_BIN", getAssignedPortBinPath)

	isOneShot := !shouldHotReload && testLabel != ""

	serviceSpecs, err := readVersionedServiceSpecs(serviceSpecsPath)
	must(err)

	/*if *allowSvcctl {
		addr := net.Listen(network, address)
	}*/

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	r, err := runner.New(ctx, serviceSpecs)
	must(err)

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
			testArgs := os.Args[1:]
			testPath, err := runfiles.Rlocation(os.Getenv("SVCINIT_TEST_RLOCATION_PATH"))
			must(err)

			fmt.Println("")
			log.Printf("Executing test: %s, %s\n", testPath, strings.Join(testArgs, " "))
			testCmd = exec.CommandContext(ctx, testPath, testArgs...)
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

		if shouldHotReload && !enablePerServiceReload {
			fmt.Println()
			fmt.Println()
			fmt.Println("###########################################################################################")
			fmt.Println("  Detected that you are running under ibazel, but do not have per-service-reload enabled.")
			fmt.Println("  In this configuration, services will not be restarted when their code changes.")
			fmt.Println("  If this was unintentional, you can retry with per-service-reload enabled:")
			fmt.Println("")
			fmt.Printf("  `bazel run --@rules_itest//:enable_per_service_reload %s`\n", testLabel)
			fmt.Println("###########################################################################################")
			fmt.Println()
			fmt.Println()
		}

		select {
		case <-ctx.Done():
			log.Println("Shutting down services.")
			_, err := r.StopAll()
			must(err)
			log.Println("Cleaning up.")
			return
		case ibazelCmd := <-interactiveCh:
			log.Println(ibazelCmd)

			// Restart any services as needed.
			serviceSpecs, err := readVersionedServiceSpecs(serviceSpecsPath)
			must(err)

			criticalPath, err = r.UpdateSpecsAndRestart(serviceSpecs, []byte(ibazelCmd))
			must(err)
		}
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
		s := svclib.VersionedServiceSpec{
			ServiceSpec: serviceSpec,
		}

		exePath, err := runfiles.Rlocation(s.Exe)
		if err != nil {
			return nil, err
		}
		s.Exe = exePath

		if s.HealthCheck != "" {
			healthCheckPath, err := runfiles.Rlocation(serviceSpec.HealthCheck)
			if err != nil {
				return nil, err
			}
			s.HealthCheck = healthCheckPath
		}

		if serviceSpec.VersionFile != "" {
			versionFilePath, err := runfiles.Rlocation(serviceSpec.VersionFile)
			if err != nil {
				return nil, err
			}

			version, err := os.ReadFile(versionFilePath)
			if err != nil {
				return nil, err
			}
			s.Version = string(version)
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
						setSockoptErr = setSockoptLinger(fd, syscall.SOL_SOCKET, syscall.SO_LINGER, &syscall.Linger{
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
