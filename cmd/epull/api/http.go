package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"git.fedesito.me/fedes1to/eserve/cmd/epull/clientConfig"
)

var mtlsClient *http.Client

func sendMtlsRequest[T any](subUrl string, payload any, into *T, expectedStatus ...int) error {
	return sendMtlsRequestWithToken(subUrl, payload, "", into, expectedStatus...)
}

func sendMtlsRequestWithToken[T any](subUrl string, payload any, token string, into *T, expectedStatus ...int) error {
	body, _ := json.Marshal(payload)
	request, err := http.NewRequest("POST", clientConfig.Settings.Server+subUrl, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	return sendRequest(request, into, mtlsClient, expectedStatus...)
}

// GET variant: reports the status code so callers can special-case 404
func sendMtlsGet[T any](subUrl string, into *T) (int, error) {
	request, err := http.NewRequest("GET", clientConfig.Settings.Server+subUrl, nil)
	if err != nil {
		return 0, err
	}

	response, err := mtlsClient.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return response.StatusCode, nil
	}

	if response.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return response.StatusCode, fmt.Errorf("Invalid response, got code %v without body due to: %w",
				response.StatusCode, err)
		}
		return response.StatusCode, fmt.Errorf("Invalid response, got code %v with body:\n%v",
			response.StatusCode, string(bodyBytes))
	}

	// decode json to struct
	if err := json.NewDecoder(response.Body).Decode(into); err != nil {
		return response.StatusCode, err
	}

	return response.StatusCode, nil
}

// raw GET for binary downloads; caller owns the response and must close the body
func sendMtlsGetRaw(subUrl string) (*http.Response, error) {
	request, err := http.NewRequest("GET", clientConfig.Settings.Server+subUrl, nil)
	if err != nil {
		return nil, err
	}
	return mtlsClient.Do(request)
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
