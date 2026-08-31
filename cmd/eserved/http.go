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
	"strings"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/admin"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/api"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/chroot"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
	"git.fedesito.me/fedes1to/eserve/internal/cli"
	"git.fedesito.me/fedes1to/eserve/internal/gpg"
	"git.fedesito.me/fedes1to/eserve/internal/urls"
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
		{Name: "signing key", Function: gpg.EnsureKey},
	}
	if err := cli.MustInit(steps); err != nil {
		return err
	}

	if adminEnabled {
		adminMux := http.NewServeMux()
		adminMux.HandleFunc(urls.CreateTokenSuburl, admin.PostCreateToken)
		adminMux.HandleFunc(urls.TokensListSuburl, admin.PostListTokens)
		adminMux.HandleFunc(urls.RevokeMachineSuburl, admin.PostRevokeMachine)
		adminMux.HandleFunc(urls.MachinesListSuburl, admin.PostListMachines)
		adminMux.HandleFunc(urls.BuildStartSuburl, admin.PostStartBuild)
		adminMux.HandleFunc(urls.JobsListSuburl, admin.PostListJobs)
		adminMux.HandleFunc(urls.AdminJobsCancelSuburl, admin.PostAdminCancelJob)
		adminMux.HandleFunc(urls.AdminJobsStreamSuburl, admin.PostAdminJobStream)
		adminMux.HandleFunc(urls.FlavorApplySuburl, admin.PostApplyFlavor)
		adminMux.HandleFunc(urls.BinaryUploadSuburl, admin.PostUploadBinary)
		adminMux.HandleFunc(urls.BinaryListSuburl, admin.PostListBinaries)

		os.Remove(urls.SocketPath)
		unixSocket, err := net.Listen("unix", urls.SocketPath)
		if err != nil {
			return err
		}
		defer unixSocket.Close()
		os.Chmod(urls.SocketPath, 0600)
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
	apiMux.HandleFunc(urls.IdentitySuburl, api.PostIdentity)
	apiMux.HandleFunc(urls.CaSuburl, api.GetCa)
	apiMux.Handle(urls.ProvisionSuburl, requireClientCert(http.HandlerFunc(api.PostProvision)))
	apiMux.Handle(urls.SyncSuburl, requireClientCert(http.HandlerFunc(api.PostSync)))
	apiMux.Handle(urls.CheckSyncSuburl, requireClientCert(http.HandlerFunc(api.PostCheckSync)))
	apiMux.Handle(urls.StagesSuburl, requireClientCert(http.HandlerFunc(api.GetStages)))
	apiMux.Handle(urls.JobsStreamSuburl, requireClientCert(http.HandlerFunc(api.GetJobStream)))
	apiMux.Handle(urls.JobsCancelSuburl, requireClientCert(http.HandlerFunc(api.PostCancelJob)))
	apiMux.Handle(urls.BinarySuburl, requireClientCert(http.HandlerFunc(api.GetBinary)))
	apiMux.Handle(urls.BinaryManifestSuburl, requireClientCert(http.HandlerFunc(api.GetBinaryManifest)))
	apiMux.Handle(urls.SigningKeySuburl, requireClientCert(http.HandlerFunc(api.GetSigningKey)))
	// portage can't do mTLS; binaries/ stays out, its mTLS-only at /api/v1/binary
	apiMux.Handle(urls.PkgsSuburl+"/", pkgsHandler(serverConfig.Settings.RepoBase))

	apiServer := &http.Server{
		Addr:      serverConfig.Settings.ListenAddr,
		Handler:   apiMux,
		TLSConfig: tlsCfg,
	}
	return apiServer.ListenAndServeTLS("", "")
}

// serves the binhost dirs off repo_base, minus the binaries dir
func pkgsHandler(repoBase string) http.Handler {
	fileServer := http.FileServer(http.Dir(repoBase))
	return http.StripPrefix(urls.PkgsSuburl+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// strip the leading slash, the toolchain's StripPrefix drops it
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "binaries" || strings.HasPrefix(p, "binaries/") {
			http.NotFound(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	}))
}
