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

	testLabel := flags.String("svc.test-label", "", "Label for the test to run, if any. If none, test will not be executed.")
	serviceDefinitionsPath := flags.String("svc.definitions-path", "", "File defining which services to run")
	allowSvcctl := flags.Bool("svc.allow-svcctl", false, "If true, spawns a server to handle svcctl commands")
	_ = allowSvcctl

	shouldHotReload := os.Getenv("IBAZEL_NOTIFY_CHANGES") == "y"

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

	isOneShot := !shouldHotReload && *testLabel != ""

	data, err := os.ReadFile(*serviceDefinitionsPath)
	must(err)

	var services map[string]svclib.Service
	err = json.Unmarshal(data, &services)
	must(err)

	for k, v := range services {
		fmt.Println(k, v)
	}

	r := runner.New(services)
	err = r.StartAll()
	must(err)

	/*if *allowSvcctl {
		addr := net.Listen(network, address)
	}*/

	for {
		var testCmd *exec.Cmd
		var testErr error
		if *testLabel != "" {
			log.Printf("Executing test: %s\n", strings.Join(testArgs, " "))
			// Wrap in a shell to handle sh_test
			testCmd = exec.Command("/bin/sh",
				append([]string{"-c", "--"}, testArgs...)...)
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

		if *testLabel != "" {
			_, err = reportWriter.Write([]byte(fmt.Sprintf("%s\t%s\t%s\n",
				*testLabel, testCmd.ProcessState.UserTime(), testCmd.ProcessState.SystemTime())))
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
		data, err := os.ReadFile(*serviceDefinitionsPath)
		must(err)
		fmt.Println(string(data))

		var services map[string]svclib.Service
		err = json.Unmarshal(data, &services)
		must(err)

		err = r.UpdateDefinitions(services)
		must(err)
	}
}
