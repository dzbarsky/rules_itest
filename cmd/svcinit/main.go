package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
	"time"

	"rules_itest/runner"
	"rules_itest/svclib"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	fmt.Println(os.Args)
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

	// If we are under `bazel run` for a service group, we may not have TEST_TMPDIR set.
	tmpdir := os.Getenv("TEST_TMPDIR")
	if tmpdir == "" {
		var err error
		tmpdir, err = os.MkdirTemp("", testLabel)
		must(err)
	}
	os.Setenv("TMPDIR", tmpdir)

	isOneShot := !shouldHotReload && testLabel != ""

	serviceSpecs, err := readVersionedServiceSpecs(*serviceSpecsPath)
	must(err)

	for k, v := range serviceSpecs {
		fmt.Println(k, v)
	}

	r := runner.New(serviceSpecs)
	err = r.StartAll()
	must(err)

	/*if *allowSvcctl {
		addr := net.Listen(network, address)
	}*/

	for {
		var testCmd *exec.Cmd
		var testErr error
		if testLabel != "" {
			log.Printf("Executing test: %s\n", strings.Join(testArgs, " "))
			testCmd = exec.Command(testArgs[0], testArgs[1:]...)
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
		// API is                 NewWriter(output io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) *Writer
		reportWriter := tabwriter.NewWriter(os.Stdout, 0, 4, 4, ' ', 0)
		reportWriter.Write([]byte("Target\tUser Time\tSystem Time\n"))

		if isOneShot {
			states, err := r.StopAll()
			must(err)
			for label, state := range states {
				_, err = reportWriter.Write([]byte(fmt.Sprintf("%s\t%s\t%s\n",
					label, state.UserTime(), state.SystemTime())))
				must(err)
			}
		}

		if testLabel != "" {
			_, err = reportWriter.Write([]byte(fmt.Sprintf("%s\t%s\t%s\n",
				testLabel, testCmd.ProcessState.UserTime(), testCmd.ProcessState.SystemTime())))
			must(err)
		}

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

		<-interactiveCh

		// Restart any services as needed.
		serviceSpecs, err := readVersionedServiceSpecs(*serviceSpecsPath)
		must(err)

		err = r.UpdateSpecsAndRestart(serviceSpecs)
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

	testTmpdir := os.Getenv("TMPDIR")

	versionedServiceSpecs := make(map[string]svclib.VersionedServiceSpec, len(serviceSpecs))
	for label, serviceSpec := range serviceSpecs {
		version, err := os.ReadFile(serviceSpec.VersionFile)
		if err != nil {
			return nil, err
		}

		for i, arg := range serviceSpec.Args {
			serviceSpec.Args[i] = strings.ReplaceAll(arg, "$$TMPDIR", testTmpdir)
		}

		versionedServiceSpecs[label] = svclib.VersionedServiceSpec{
			ServiceSpec: serviceSpec,
			Version:     string(version),
		}
	}
	return versionedServiceSpecs, nil
}
