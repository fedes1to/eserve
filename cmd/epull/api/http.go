package api

import (
	"crypto/tls"
	"net/http"

	"git.fedesito.me/fedes1to/eserve/cmd/epull/clientConfig"
)

var mtlsClient *http.Client

// config must be populated to call this method
func InitializeMtlsClient(insecure bool) error {
	certificate, err := tls.LoadX509KeyPair(clientConfig.Settings.CertPath, clientConfig.Settings.PrivatePEMPath)
	if err != nil {
		return err
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
