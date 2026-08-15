package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	clientConfig "git.fedesito.me/fedes1to/eserve/cmd/epull/config"
	"git.fedesito.me/fedes1to/eserve/internal/initialization"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
	"git.fedesito.me/fedes1to/eserve/internal/sysinfo"
)

var mtlsClient *http.Client

// config must be populated to call this method
func initializeMtlsClient(insecure bool) error {
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

func handleIdentification(token string, server string, insecure bool) error {
	// making the certs and CSR
	_, privKey, keyError := ed25519.GenerateKey(rand.Reader)
	if keyError != nil {
		return keyError
	}

	hostname, _ := os.Hostname()
	csrTemplate := &x509.CertificateRequest{
		// use hostname since its good enough
		Subject: pkix.Name{CommonName: hostname},
	}
	csrDER, csrError := x509.CreateCertificateRequest(rand.Reader, csrTemplate, privKey)
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	if csrError != nil {
		return csrError
	}

	keyDER, _ := x509.MarshalPKCS8PrivateKey(privKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	// request to /api/v1/identity
	var client *http.Client
	if insecure {
		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
					MinVersion:         tls.VersionTLS13,
				},
			},
		}
	} else {
		client = http.DefaultClient
	}
	payload := protocol.IdentificationRequest{Csr: string(csrPEM)}
	body, _ := json.Marshal(payload)

	request, _ := http.NewRequest("POST", server+"/api/v1/identity", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	var identificationResponse protocol.IdentificationResponse
	if requestError := sendRequest(request, &identificationResponse, client); requestError != nil {
		return requestError
	}

	// checks to be nice n shit
	if identificationResponse.CN != hostname {
		return fmt.Errorf(
			"Hostname mismatch! expected %v, received %v\n",
			hostname,
			identificationResponse.CN)
	}

	validUntil, parseError := time.Parse(time.RFC3339, identificationResponse.ValidUntil)
	if parseError != nil {
		fmt.Fprintln(os.Stderr, "Couldn't parse valid_until field,", parseError)
	} else {
		daysLeft := time.Until(validUntil).Hours() / 24
		if daysLeft < 30 {
			log.Printf("Careful! Certificates expire in %v days\n", daysLeft)
		}
	}

	clientConfig.Settings.PrivatePEMPath = clientConfig.ClientConfigPath + hostname + ".key"
	clientConfig.Settings.CertPath = clientConfig.ClientConfigPath + hostname + ".crt"
	clientConfig.Settings.Server = server

	// assuming everything went well here, so we save the request
	os.WriteFile(clientConfig.Settings.CertPath, []byte(identificationResponse.Certificate), 0644)
	os.WriteFile(clientConfig.Settings.PrivatePEMPath, keyPEM, 0600)

	if saveError := clientConfig.SaveClientSettings(); saveError != nil {
		return saveError
	}

	return nil
}

func sendRequest[T any](request *http.Request, into *T, httpClient *http.Client) error {
	response, responseError := httpClient.Do(request)
	if responseError != nil {
		return responseError
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		bodyBytes, ioError := io.ReadAll(response.Body)
		if ioError != nil {
			return fmt.Errorf("Invalid response, got code %v without body due to: %w",
				response.StatusCode, ioError)
		}
		return fmt.Errorf("Invalid response, got code %v with body:\n%v",
			response.StatusCode, string(bodyBytes))
	}

	// decode json to struct
	jsonError := json.NewDecoder(response.Body).Decode(into)
	if jsonError != nil {
		return jsonError
	}

	return nil
}

func sendMtlsRequest[T any](subUrl string, payload any, into *T) error {
	body, _ := json.Marshal(payload)
	request, _ := http.NewRequest("POST", clientConfig.Settings.Server+subUrl, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	return sendRequest(request, into, mtlsClient)
}

func handleProvisioning(flavor string) error {
	// Provisioning
	cpuSubarch, subarchError := sysinfo.GetCpuMarch()
	if subarchError != nil {
		return subarchError
	}

	cpuChost, chostError := sysinfo.GetCpuChost()
	if chostError != nil {
		return chostError
	}

	log.Printf("Found arch %v with subarch %v\n", cpuChost, cpuSubarch)

	// construct JSON provisioning payload
	payload := protocol.ProvisionRequest{Arch: cpuChost, SubArch: cpuSubarch, Flavor: flavor}
	var provisionResponse protocol.ProvisionResponse
	requestError := sendMtlsRequest("/api/v1/provision", payload, &provisionResponse)
	if requestError != nil {
		return requestError
	}

	return nil
}

func handleRegistration(token string, server string, flavor string, insecure bool) (error, int) {
	log.Printf("Registering on server: %v with flavor %v", server, flavor)

	return initialization.MustRegister([]initialization.InitStep{
		{Name: "identity", Function: func() error { return handleIdentification(token, server, insecure) }},
		{Name: "mtls client", Function: func() error { return initializeMtlsClient(insecure) }},
		{Name: "provisioning", Function: func() error { return handleProvisioning(flavor) }},
	})

}
