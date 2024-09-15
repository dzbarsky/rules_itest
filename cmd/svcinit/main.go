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
	"slices"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/bazelbuild/rules_go/go/runfiles"

	"rules_itest/logger"
	"rules_itest/runner"
	"rules_itest/svcctl"
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

	// Set up the environment properly so child processes can find their runfiles.
	runfilesEnv, err := runfiles.Env()
	must(err)
	for _, kv := range runfilesEnv {
		parts := strings.SplitN(kv, "=", 2)
		os.Setenv(parts[0], parts[1])
	}

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

	// Provide a TMPDIR if one is not set.
	if _, ok := os.LookupEnv("TMPDIR"); !ok {
		os.Setenv("TMPDIR", os.TempDir())
	}

	getAssignedPortBinPath, err := runfiles.Rlocation(os.Getenv("SVCINIT_GET_ASSIGNED_PORT_BIN_RLOCATION_PATH"))
	must(err)
	os.Setenv("GET_ASSIGNED_PORT_BIN", getAssignedPortBinPath)

	isOneShot := !shouldHotReload && testLabel != ""

	unversionedSpecs, err := readServiceSpecs(serviceSpecsPath)
	must(err)

	ports, err := assignPorts(unversionedSpecs)
	must(err)

	replacements := getReplacementMap(ports)

	serviceSpecs, err := augmentServiceSpecs(unversionedSpecs, ports, replacements)
	must(err)

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	r, err := runner.New(ctx, serviceSpecs)
	must(err)

	servicesErrCh := make(chan error, len(serviceSpecs))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	must(err)

	go func() {
		defer listener.Close()
		err := svcctl.Serve(ctx, listener, r, ports, servicesErrCh)
		if err != nil {
			log.Fatalf("svcctl.Serve: %v", err)
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	portString := strconv.Itoa(port)
	os.Setenv("SVCCTL_PORT", portString)

	if testLabel == "" {
		err = os.WriteFile("/tmp/svcctl_port", []byte(portString), 0600)
		must(err)
		defer os.Remove("/tmp/svcctl_port")
	}

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

	criticalPath, err := r.StartAll(servicesErrCh)
	if errors.Is(err, context.Canceled) {
		_, err := r.StopAll()
		must(err)
		return
	}
	must(err)

	// API is NewWriter(output io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) *Writer
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
		testErrCh := make(chan error, 1)
		if testLabel != "" {
			testArgs := os.Args[1:]

			for i := range testArgs {
				testArgs[i] = fixupReplacementOccurrences(testArgs[i], replacements)
			}

			testPath, err := runfiles.Rlocation(os.Getenv("SVCINIT_TEST_RLOCATION_PATH"))
			must(err)

			testEnv, err := buildTestEnv(ports)
			must(err)

			fmt.Println("")
			log.Printf("Executing test: %s, %s\n", testPath, strings.Join(testArgs, " "))
			testCmd = exec.CommandContext(ctx, testPath, testArgs...)
			testCmd.Env = testEnv
			testCmd.Stdout = os.Stdout
			testCmd.Stderr = os.Stderr

			testStartTime := time.Now()

			if err := testCmd.Start(); err != nil {
				panic(err)
			}

			go func() {
				testErrCh <- testCmd.Wait()

				testDuration := time.Since(testStartTime)
				log.Printf("Test duration: %s\n", testDuration)
			}()
		}

		fmt.Println()

		select {
		case testErr := <-testErrCh:
			if testErr != nil {
				log.Printf("Encountered error during test run: %s\n", testErr)
				if isOneShot {
					os.Exit(1)
				}
			}
		case serviceErr := <-servicesErrCh:
			log.Print(serviceErr)
			if isOneShot {
				log.Fatal("Service exited uncleanly, marking test as failed.\n\n")
			}
		}

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
			unversionedSpecs, err := readServiceSpecs(serviceSpecsPath)
			must(err)

			serviceSpecs, err := augmentServiceSpecs(unversionedSpecs, ports, replacements)
			must(err)

			// TODO(zbarsky): what is the right behavior here when services are crashing in ibazel mode?
			for range servicesErrCh {
			} // Drain the channel
			criticalPath, err = r.UpdateSpecsAndRestart(serviceSpecs, servicesErrCh, []byte(ibazelCmd))
			must(err)
		}
	}
}

func readServiceSpecs(
	path string,
) (
	map[string]svclib.ServiceSpec, error,
) {
	data, err := os.ReadFile(path)
	must(err)

	var serviceSpecs map[string]svclib.ServiceSpec
	err = json.Unmarshal(data, &serviceSpecs)
	return serviceSpecs, err
}

func assignPorts(
	serviceSpecs map[string]svclib.ServiceSpec,
) (
	svclib.Ports, error,
) {
	var toClose []net.Listener
	ports := svclib.Ports{}

	for label, spec := range serviceSpecs {
		namedPorts := slices.Clone(spec.NamedPorts)
		if spec.AutoassignPort {
			namedPorts = append(namedPorts, "")
		}

		// Note, this can cause collisions. So be careful!
		// To avoid port collisions, set the `so_reuseport_aware` option on the service definition
		// and use the SO_REUSEPORT socket option in your services.
		for _, portName := range namedPorts {
			// We do a bit of a dance here to set SO_LINGER to 0. For details, see
			// https://stackoverflow.com/questions/71975992/what-really-is-the-linger-time-that-can-be-set-with-so-linger-on-sockets
			lc := net.ListenConfig{
				Control: func(network, address string, conn syscall.RawConn) error {
					var setSockoptErr error
					err := conn.Control(func(fd uintptr) {
						setSockoptErr = setSockoptsForPortAssignment(fd, &syscall.Linger{
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

			qualifiedPortName := label
			if portName != "" {
				qualifiedPortName += ":" + portName
			}

			fmt.Printf("Assigning port %s to %s\n", port, qualifiedPortName)
			ports.Set(qualifiedPortName, port)

			if !spec.SoReuseportAware {
				toClose = append(toClose, listener)
			}
		}
	}

	for _, listener := range toClose {
		err := listener.Close()
		if err != nil {
			return nil, err
		}
	}

	// Complete hack - we have observed that the ports may not be ready immediately after closing, even with SO_LINGER set to 0.
	// Give the kernel a bit of time to figure out what we've done.
	time.Sleep(10 * time.Millisecond)

	serializedPorts, err := ports.Marshal()
	if err != nil {
		return nil, err
	}
	os.Setenv("ASSIGNED_PORTS", string(serializedPorts))
	return ports, nil
}

func getReplacementMap(ports svclib.Ports) []Replacement {
	tmpDir := os.Getenv("TMPDIR")
	testTmpDir := os.Getenv("TEST_TMPDIR")
	socketDir := os.Getenv("SOCKET_DIR")

	replacements := make([]Replacement, 0, 3+len(ports))
	replacements = append(replacements,
		Replacement{Old: "$${TMPDIR}", New: tmpDir},
		Replacement{Old: "$${TEST_TMPDIR}", New: testTmpDir},
		Replacement{Old: "$${SOCKET_DIR}", New: socketDir},
	)
	for label, port := range ports {
		replacements = append(replacements, Replacement{
			Old: "$${" + label + "}",
			New: port,
		})
	}

	return replacements
}

func fixupReplacementOccurrences(value string, replacements []Replacement) string {
	for _, r := range replacements {
		value = strings.ReplaceAll(value, r.Old, r.New)
	}
	return value
}

func augmentServiceSpecs(
	serviceSpecs map[string]svclib.ServiceSpec,
	ports svclib.Ports,
	replacements []Replacement,
) (
	map[string]svclib.VersionedServiceSpec, error,
) {

	versionedServiceSpecs := make(map[string]svclib.VersionedServiceSpec, len(serviceSpecs))
	for label, serviceSpec := range serviceSpecs {
		s := svclib.VersionedServiceSpec{
			ServiceSpec: serviceSpec,
		}

		if s.Type == "group" {
			versionedServiceSpecs[label] = s
			continue
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

		if s.AutoassignPort {
			port := ports[s.Label]
			for i := range s.ServiceSpec.Args {
				s.Args[i] = strings.ReplaceAll(s.Args[i], "$${PORT}", port)
			}
			s.HttpHealthCheckAddress = strings.ReplaceAll(s.HttpHealthCheckAddress, "$${PORT}", port)
			for i := range s.ServiceSpec.HealthCheckArgs {
				s.HealthCheckArgs[i] = strings.ReplaceAll(s.HealthCheckArgs[i], "$${PORT}", port)
			}
			for k, v := range s.Env {
				s.Env[k] = strings.ReplaceAll(v, "$${PORT}", port)
			}
		}

		versionedServiceSpecs[label] = s
	}

	for label, spec := range versionedServiceSpecs {
		spec.HttpHealthCheckAddress = fixupReplacementOccurrences(spec.HttpHealthCheckAddress, replacements)
		for i := range spec.Args {
			spec.Args[i] = fixupReplacementOccurrences(spec.Args[i], replacements)
		}
		for i := range spec.HealthCheckArgs {
			spec.HealthCheckArgs[i] = fixupReplacementOccurrences(spec.HealthCheckArgs[i], replacements)
		}
		for k, v := range spec.Env {
			spec.Env[k] = fixupReplacementOccurrences(v, replacements)
		}
		versionedServiceSpecs[label] = spec
	}

	return versionedServiceSpecs, nil
}

type Replacement struct {
	Old string
	New string
}

func buildTestEnv(ports svclib.Ports) ([]string, error) {
	testEnvPath, err := runfiles.Rlocation(os.Getenv("SVCINIT_TEST_ENV_RLOCATION_PATH"))
	if err != nil {
		panic(err)
	}

	testEnvData, err := os.ReadFile(testEnvPath)
	if err != nil {
		panic(err)
	}

	env := map[string]string{}
	err = json.Unmarshal(testEnvData, &env)
	if err != nil {
		panic(err)
	}

	replacements := make([]Replacement, 0, len(ports))
	for label, port := range ports {
		replacements = append(replacements, Replacement{
			Old: "$${" + label + "}",
			New: port,
		})
	}

	replaceAllPorts := func(s string) string {
		for _, r := range replacements {
			s = strings.ReplaceAll(s, r.Old, r.New)
		}
		return s
	}

	// Note, this can technically specify the same var multiple times.
	// Last one wins - hope that's what you wanted!
	baseEnv := os.Environ()
	for k, v := range env {
		baseEnv = append(baseEnv, k+"="+replaceAllPorts(v))
	}

	return baseEnv, nil
}
