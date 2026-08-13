package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
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
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

func handleIdentification(token string, server string) error {
	// making the certs and CSR
	_, privKey, keyError := ed25519.GenerateKey(nil)
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
	payload := protocol.IdentificationRequest{Csr: string(csrPEM)}
	body, _ := json.Marshal(payload)

	request, _ := http.NewRequest("POST", server+"/api/v1/identity", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	response, responseError := http.DefaultClient.Do(request)
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
	var identificationResponse protocol.IdentificationResponse
	jsonError := json.NewDecoder(response.Body).Decode(&identificationResponse)
	if jsonError != nil {
		return jsonError
	}

	// checks to be nice n shit
	if identificationResponse.CN != hostname {
		return fmt.Errorf(
			"Hostname mismatch! expected %v, received %v\n",
			hostname,
			identificationResponse.CN)
	}

	validUntil, _ := time.Parse(time.RFC3339, identificationResponse.ValidUntil)
	daysLeft := time.Until(validUntil).Hours() / 24

	if daysLeft < 30 {
		log.Printf("Careful! Certificates expire in %v days\n", daysLeft)
	}

	clientConfig.Settings.PrivatePEMPath = clientConfig.ClientConfigPath + hostname + ".key"
	clientConfig.Settings.CertPath = clientConfig.ClientConfigPath + hostname + ".crt"
	clientConfig.Settings.CAPath = clientConfig.ClientConfigPath + "ca.crt"
	clientConfig.Settings.Server = server

	// assuming everything went well here, so we save the request
	os.WriteFile(clientConfig.Settings.CertPath, []byte(identificationResponse.Certificate), 0644)
	os.WriteFile(clientConfig.Settings.CAPath, []byte(identificationResponse.CA), 0644)
	os.WriteFile(clientConfig.Settings.PrivatePEMPath, keyPEM, 0600)

	if saveError := clientConfig.SaveClientSettings(); saveError != nil {
		return saveError
	}

	return nil

}

func handleProvisioning(flavor string) error {
	// Provisioning
	cpuSubarch, subarchError := getCpuSubarch()

	if subarchError != nil {
		return subarchError
	}

	log.Println("Found subarch:", cpuSubarch)

	// construct JSON provisioning payload
	//payload := protocol.ProvisionRequest{SubArch: cpuSubarch, Flavor: flavor}
	return nil
}

func handleRegistration(token string, server string, flavor string) (error, int) {
	log.Printf("Registering on server: %v with flavor %v", server, flavor)

	if identifyError := handleIdentification(token, server); identifyError != nil {
		return identifyError, 1
	}
	if provisioningError := handleProvisioning(flavor); provisioningError != nil {
		return provisioningError, 1
	}
	return nil, 0
}
