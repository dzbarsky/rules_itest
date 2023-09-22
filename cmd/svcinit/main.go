package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"rules_itest/svclib"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

type ServiceCommand struct {
	Service svclib.Service
	Cmd     *exec.Cmd
}

func main() {
	flags := flag.NewFlagSet("svcinit", flag.ExitOnError)

	serviceDefinitionsPath := flags.String("svc.definitions-path", "", "File defining which services to run")

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

	var serviceCmds []ServiceCommand

	for _, service := range services {
		cmd := exec.Command(service.Exe, service.Args...)
		for k, v := range service.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Start()
		must(err)
		defer cmd.Process.Kill()

		if service.HttpHealthCheckAddress != "" {
			waitUntilHealthy(cmd, service)
			fmt.Println(cmd.ProcessState)
		}

		serviceCmds = append(serviceCmds, ServiceCommand{
			Service: service,
			Cmd:     cmd,
		})
	}

	log.Printf("Executing command: %s\n", strings.Join(testArgs, " "))
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
	log.Printf("Test resource utilization: User: %v System: %v",
		testCmd.ProcessState.UserTime(), testCmd.ProcessState.SystemTime())

	for _, serviceCmd := range serviceCmds {
		serviceCmd.Cmd.Process.Kill()
		serviceCmd.Cmd.Wait()

		state := serviceCmd.Cmd.ProcessState
		if state == nil {
			log.Print("TODO: nil process state")
			continue
		}
		log.Printf("%s resource utilization: User: %v System: %v",
			serviceCmd.Service.Label, state.UserTime(), state.SystemTime())
	}

	if testErr != nil {
		log.Printf("Encountered error during test run: %s\n", testErr)
		os.Exit(1)
	}
}

func waitUntilHealthy(cmd *exec.Cmd, service svclib.Service) bool {
	exitedCh := make(chan struct{})
	go func() {
		status, err := cmd.Process.Wait()
		must(err)
		if status.Exited() {
			close(exitedCh)
		}
	}()
	for {
		select {
		case <-exitedCh:
			return false
		default:
		}

		log.Printf("Healthchecking %s at %s...\n", service.Label, service.HttpHealthCheckAddress)
		resp, err := http.DefaultClient.Get(service.HttpHealthCheckAddress)
		if resp != nil {
			defer resp.Body.Close()
		}
		if err == nil {
			log.Printf("%s healthy!\n", service.Label)
			return true
		}
		fmt.Println(err)
		time.Sleep(200 * time.Millisecond)
		fmt.Println("status", cmd.Process)
	}
}
