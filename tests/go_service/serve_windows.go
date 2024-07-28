//go:build windows

package main

import "net/http"

func serve(port string, soReuseport bool) {
	if soReuseport {
		panic("SO_REUSEPORT not supported on Windows!")
	}
	http.ListenAndServe("127.0.0.1:"+port, nil)
}
