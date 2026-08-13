package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	serverConfig "git.fedesito.me/fedes1to/eserve/cmd/eserved/config"
)

var socketPath = "/run/eserved.sock"

func clientIP(r *http.Request) string {
	proxyIpHeader := r.Header.Get("X-Forwarded-For")

	if proxyIpHeader == "" {
		return r.RemoteAddr
	}

	return strings.Split(proxyIpHeader, ",")[0] + " (via " + r.RemoteAddr + ")"
}

func requireClientCert(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			http.Error(w, "client certificate required", http.StatusUnauthorized)
			return
		}
		// TODO: check fingerprint from machines.json
		next.ServeHTTP(w, r)
	})
}

func serveHTTP(admin bool) error {
	if admin {
		adminMux := http.NewServeMux()
		adminMux.HandleFunc("/admin/v1/create_token", postCreateToken)

		os.Remove(socketPath)
		unixSocket, listenError := net.Listen("unix", socketPath)
		if listenError != nil {
			return listenError
		}
		defer unixSocket.Close()
		os.Chmod(socketPath, 0600)
		adminServer := &http.Server{Handler: adminMux}

		go func() {
			if serveError := adminServer.Serve(unixSocket); serveError != nil {
				fmt.Fprintln(os.Stderr, "Failed to serve admin socket,", serveError)
			}
		}()
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{serverConfig.TlsCertificate},
		ClientCAs:    serverConfig.CaPool,
		ClientAuth:   tls.VerifyClientCertIfGiven,
		MinVersion:   tls.VersionTLS13,
	}

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/v1/provision", postProvision)

	outerMux := http.NewServeMux()
	outerMux.HandleFunc("/api/v1/identity", postIdentity) // specifically here we dont mTLS
	outerMux.Handle("/", requireClientCert(apiMux))

	apiServer := &http.Server{
		Addr:      serverConfig.Settings.ListenAddr,
		Handler:   outerMux,
		TLSConfig: tlsCfg,
	}
	return apiServer.ListenAndServeTLS("", "")
}
