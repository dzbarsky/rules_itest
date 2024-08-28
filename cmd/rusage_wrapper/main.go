package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		syscall.SIGHUP,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
		syscall.SIGPIPE,
		syscall.SIGALRM,
		syscall.SIGTSTP,
		syscall.SIGCONT,
	)

	rusageFile := os.Args[1]
	bin := os.Args[2]
	args := os.Args[3:]

	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	must(err)

	go func() {
		for sig := range signals {
			if sig == syscall.SIGUSR2 {
				sig = syscall.SIGKILL
			}
			err := cmd.Process.Signal(sig)
			must(err)
		}
	}()

	err = cmd.Wait()
	if _, ok := err.(*exec.ExitError); !ok {
		panic(err)
	}

	var usage syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_CHILDREN, &usage)

	data, err := json.Marshal(usage)
	must(err)

	err = os.WriteFile(rusageFile, data, 0600)
	must(err)

	os.Exit(cmd.ProcessState.ExitCode())
}
