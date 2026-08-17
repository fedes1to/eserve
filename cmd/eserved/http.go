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
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/chroot"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
	"git.fedesito.me/fedes1to/eserve/internal/cli"
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
	steps := []cli.InitStep{
		{Name: "settings", Function: serverConfig.InitializeServerSettings},
		{Name: "stage", Function: storage.InitializeStageFolder},
		{Name: "gcc info", Function: chroot.InitializeGccInfo},
		{Name: "ca certificate", Function: storage.LoadCaCertificate},
		{Name: "tls certificate", Function: storage.LoadTlsCertificates},
		{Name: "tokens", Function: storage.LoadTokens},
		{Name: "machines", Function: storage.LoadMachines},
	}
	if err := cli.MustInit(steps); err != nil {
		return err
	}

	if adminEnabled {
		adminMux := http.NewServeMux()
		adminMux.HandleFunc("/admin/v1/create_token", admin.PostCreateToken)

		os.Remove(admin.SocketPath)
		unixSocket, err := net.Listen("unix", admin.SocketPath)
		if err != nil {
			return err
		}
		defer unixSocket.Close()
		os.Chmod(admin.SocketPath, 0600)
		adminServer := &http.Server{Handler: adminMux}

		go func() {
			if err := adminServer.Serve(unixSocket); err != nil {
				fmt.Fprintln(os.Stderr, "Failed to serve admin socket,", err)
			}
		}()
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{storage.TlsCertificate},
		ClientCAs:    storage.CaPool,
		ClientAuth:   tls.RequestClientCert,
		MinVersion:   tls.VersionTLS13,
	}

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/v1/identity", api.PostIdentity)
	apiMux.Handle("/api/v1/provision", requireClientCert(http.HandlerFunc(api.PostProvision)))
	apiMux.Handle("/api/v1/stages", requireClientCert(http.HandlerFunc(api.GetStages)))

	apiServer := &http.Server{
		Addr:      serverConfig.Settings.ListenAddr,
		Handler:   apiMux,
		TLSConfig: tlsCfg,
	}
	return apiServer.ListenAndServeTLS("", "")
}
