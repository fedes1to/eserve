package main

import (
	"net/http"
	"strings"
)

func clientIP(r *http.Request) string {
	proxyIpHeader := r.Header.Get("X-Forwarded-For")

	if proxyIpHeader == "" {
		return r.RemoteAddr
	}

	return strings.Split(proxyIpHeader, ",")[0] + " (via " + r.RemoteAddr + ")"
}

func serveHTTP(address string) {
	http.HandleFunc("/api/v1/identity", postIdentity)
	http.ListenAndServe(address, nil)
}
