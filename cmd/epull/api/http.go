package api

import (
	"crypto/tls"
	"net/http"

	clientConfig "git.fedesito.me/fedes1to/eserve/cmd/epull/config"
)

var mtlsClient *http.Client

// config must be populated to call this method
func InitializeMtlsClient(insecure bool) error {
	certificate, certificateError := tls.LoadX509KeyPair(clientConfig.Settings.CertPath, clientConfig.Settings.PrivatePEMPath)
	if certificateError != nil {
		return certificateError
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS13,
	}

	if insecure {
		tlsConfig.InsecureSkipVerify = true
	}

	mtlsClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return nil
}
