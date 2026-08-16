package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/admin"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/api"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
)

func requireClientCert(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			http.Error(w, "client certificate required", http.StatusUnauthorized)
			return
		}
		peerCertificate := r.TLS.PeerCertificates[0]
		cn := peerCertificate.Subject.CommonName

		fingerprint := sha256.Sum256(peerCertificate.Raw)
		fingerprintHex := hex.EncodeToString(fingerprint[:])
		if !storage.MachineCertValid(cn, fingerprintHex) {
			log.Printf("%v Attempted request with invalid certificate\n", api.ClientIP(r))
			http.Error(w, "unknown or revoked machine", http.StatusUnauthorized)
			return
		}

		identity := api.ClientIdentity{
			CN:          peerCertificate.Subject.CommonName,
			Fingerprint: fingerprintHex,
		}

		ctx := context.WithValue(r.Context(), api.CtxKeyIdentity, identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func serveHTTP(adminEnabled bool) error {
	if adminEnabled {
		adminMux := http.NewServeMux()
		adminMux.HandleFunc("/admin/v1/create_token", admin.PostCreateToken)

		os.Remove(admin.SocketPath)
		unixSocket, listenError := net.Listen("unix", admin.SocketPath)
		if listenError != nil {
			return listenError
		}
		defer unixSocket.Close()
		os.Chmod(admin.SocketPath, 0600)
		adminServer := &http.Server{Handler: adminMux}

		go func() {
			if serveError := adminServer.Serve(unixSocket); serveError != nil {
				fmt.Fprintln(os.Stderr, "Failed to serve admin socket,", serveError)
			}
		}()
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{storage.TlsCertificate},
		ClientCAs:    storage.CaPool,
		ClientAuth:   tls.VerifyClientCertIfGiven,
		MinVersion:   tls.VersionTLS13,
	}

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/v1/provision", api.PostProvision)

	outerMux := http.NewServeMux()
	outerMux.HandleFunc("/api/v1/identity", api.PostIdentity) // specifically here we dont mTLS
	outerMux.Handle("/", requireClientCert(apiMux))

	apiServer := &http.Server{
		Addr:      serverConfig.Settings.ListenAddr,
		Handler:   outerMux,
		TLSConfig: tlsCfg,
	}
	return apiServer.ListenAndServeTLS("", "")
}
