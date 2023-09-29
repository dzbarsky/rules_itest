package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"rules_itest/svclib"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

type ServiceCommand struct {
	svclib.Service
	*exec.Cmd
	Version []byte

	mu     sync.Mutex
	runErr error
}

func (s *ServiceCommand) Start() {
	go func() {
		err := s.Run()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.runErr = err
	}()
}

func (s *ServiceCommand) Error() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runErr
}

func main() {
	flags := flag.NewFlagSet("svcinit", flag.ExitOnError)

	serviceDefinitionsPath := flags.String("svc.definitions-path", "", "File defining which services to run")

	isInteractive := os.Getenv("IBAZEL_NOTIFY_CHANGES") == "y"
	fmt.Println("interactive? ", isInteractive)

	interactiveCh := make(chan struct{}, 100)
	if isInteractive {
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

	data, err := os.ReadFile(*serviceDefinitionsPath)
	must(err)
	fmt.Println(string(data))

	var services map[string]svclib.Service
	err = json.Unmarshal(data, &services)
	must(err)

	serviceCmds := make(map[string]*ServiceCommand)

	for _, service := range services {
		serviceCmd := startService(service)

		if service.VersionFile != "" {
			serviceCmd.Version, err = os.ReadFile(service.VersionFile)
			must(err)
		}

		serviceCmds[service.Label] = serviceCmd
	}

	for {
		log.Printf("Executing test: %s\n", strings.Join(testArgs, " "))
		testCmd := exec.Command(testArgs[0], testArgs[1:]...)
		testCmd.Stdout = os.Stdout
		testCmd.Stderr = os.Stderr

		testStartTime := time.Now()

		if err := testCmd.Start(); err != nil {
			panic(err)
		}

		testErr := testCmd.Wait()

		testDuration := time.Since(testStartTime)
		log.Printf("Test duration: %s\n", testDuration)

		fmt.Println()
		// API is                 NewWriter(output io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) *Writer
		reportWriter := tabwriter.NewWriter(os.Stdout, 0, 4, 4, ' ', 0)
		reportWriter.Write([]byte("Target\tUser Time\tSystem Time\n"))

		if !isInteractive {
			for _, serviceCmd := range serviceCmds {
				stopService(serviceCmd)
				state := serviceCmd.Cmd.ProcessState

				_, err = reportWriter.Write([]byte(fmt.Sprintf("%s\t%s\t%s\n",
					serviceCmd.Service.Label, state.UserTime(), state.SystemTime())))
				must(err)
			}
		}

		_, err = reportWriter.Write([]byte(fmt.Sprintf("%s\t%s\t%s\n",
			testArgs[0], testCmd.ProcessState.UserTime(), testCmd.ProcessState.SystemTime())))
		must(err)

		err = reportWriter.Flush()
		must(err)

		if testErr != nil {
			log.Printf("Encountered error during test run: %s\n", testErr)
			if !isInteractive {
				os.Exit(1)
			}
		}

		if !isInteractive {
			break
		}

		<-interactiveCh

		// See which services need a restart
		data, err := os.ReadFile(*serviceDefinitionsPath)
		must(err)
		fmt.Println(string(data))

		var services map[string]svclib.Service
		err = json.Unmarshal(data, &services)
		must(err)

		for _, service := range services {
			serviceCmd, ok := serviceCmds[service.Label]
			if !ok {
				serviceCmd = startService(service)
				serviceCmds[service.Label] = serviceCmd
			}

			if service.VersionFile != "" {
				version, err := os.ReadFile(service.VersionFile)
				must(err)
				if string(version) != string(serviceCmd.Version) {
					fmt.Println(service.Label + " is stale, restarting...")
					stopService(serviceCmd)
					serviceCmd = startService(service)
					serviceCmds[service.Label] = serviceCmd
				}
				serviceCmd.Version = version
			}
		}
	}
}

func startService(service svclib.Service) *ServiceCommand {
	cmd := exec.Command(service.Exe, service.Args...)
	for k, v := range service.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	serviceCmd := &ServiceCommand{
		Service: service,
		Cmd:     cmd,
	}

	if service.Type == "task" {
		err := cmd.Wait()
		must(err)
	} else {
		serviceCmd.Start()

		if service.HttpHealthCheckAddress != "" {
			waitUntilHealthy(serviceCmd)
		}
	}

	return serviceCmd
}

func stopService(serviceCmd *ServiceCommand) {
	serviceCmd.Cmd.Process.Kill()
	serviceCmd.Cmd.Wait()

	for serviceCmd.Cmd.ProcessState == nil {
		time.Sleep(5 * time.Millisecond)
	}
}

func waitUntilHealthy(serviceCmd *ServiceCommand) bool {
	for {
		if serviceCmd.Error() != nil {
			return false
		}

		//log.Printf("Healthchecking %s at %s...\n", service.Label, service.HttpHealthCheckAddress)
		resp, err := http.DefaultClient.Get(serviceCmd.HttpHealthCheckAddress)
		if resp != nil {
			defer resp.Body.Close()
		}
		if err == nil {
			log.Printf("%s healthy!\n", serviceCmd.Label)
			return true
		}

		fmt.Println(err)
		time.Sleep(200 * time.Millisecond)
	}
}
