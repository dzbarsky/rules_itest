package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

var fibSink int

func main() {
	sleepTime := flag.Duration("sleep-time", 0, "How long to sleep before binding the port")
	busyWaitTime := flag.Duration("busy-time", 0, "How long to busy-wait before binding the port")
	fileToOpen := flag.String("file-to-open", "", "A file to open to check runfiles")

	if *fileToOpen != "" {
		f, err := os.Open(*fileToOpen)
		if err != nil {
			panic(err)
		}
		f.Close()
	}
	port := flag.Int("port", 0, "Port to bind")

	flag.Parse()

	log.Println("started")
	time.Sleep(*sleepTime)
	log.Println("done sleeping")

	finishBusyWait := time.Now().Add(*busyWaitTime)
	for time.Now().Before(finishBusyWait) {
		fibSink += fib(10)
	}

	dob := time.Now()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	http.HandleFunc("/dob", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(dob.String()))
	})
	http.HandleFunc("/fib", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(strconv.Itoa(fibSink)))
	})

	http.ListenAndServe("127.0.0.1:"+strconv.Itoa(*port), nil)
}

func fib(n int) int {
	if n < 2 {
		return 1
	}
	return fib(n-1) + fib(n-2)
}
