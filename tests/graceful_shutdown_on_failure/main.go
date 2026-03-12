package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	port := os.Args[1]

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	go http.ListenAndServe("127.0.0.1:"+port, nil)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM)
	<-sigCh

	markerPath := os.Getenv("TEST_TMPDIR") + "/shutdown_marker"
	os.WriteFile(markerPath, []byte("shutdown"), 0644)
	fmt.Println("Graceful shutdown completed")
}
