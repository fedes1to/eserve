package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"

	"git.fedesito.me/fedes1to/eserve/cmd/epull/clientConfig"
)

var mtlsClient *http.Client

func sendMtlsRequest[T any](subUrl string, payload any, into *T, expectedStatus ...int) error {
	body, _ := json.Marshal(payload)
	request, _ := http.NewRequest("POST", clientConfig.Settings.Server+subUrl, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	return sendRequest(request, into, mtlsClient, expectedStatus...)
}

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
