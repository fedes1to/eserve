package main

import (
	"net/http"
)

func serveHTTP(address string) {
	http.HandleFunc("/api/v1/identity", postIdentity)
	http.ListenAndServe(address, nil)
}
