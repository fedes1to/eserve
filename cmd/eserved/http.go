package main

import (
	"log"
	"net/http"
)

func serveHTTP(address string) {
	http.HandleFunc("/", httpHandler)
	http.ListenAndServe(address, nil)
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("request from %s: %s %q", r.RemoteAddr, r.Method, r.URL)
}
