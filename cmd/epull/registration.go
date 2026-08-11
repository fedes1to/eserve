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
	"log"
	"net/http"
	"os"
	"time"

	"git.fedesito.me/fedesito/eserve/internal/config"
	"git.fedesito.me/fedesito/eserve/internal/protocol"
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

	// decode json to struct
	var identificationResponse protocol.IdentificationResponse
	jsonError := json.NewDecoder(response.Body).Decode(&identificationResponse)
	if jsonError != nil {
		return jsonError
	}

	// checks to be nice n shit
	if identificationResponse.CN != hostname {
		fmt.Errorf(
			"Hostname mismatch! continuing..., expected %v, received %v",
			hostname,
			identificationResponse.CN)
	}

	validUntil, _ := time.Parse(time.RFC3339, identificationResponse.ValidUntil)
	daysLeft := validUntil.Sub(time.Now()).Hours() / 24

	if daysLeft < 30 {
		fmt.Errorf("Careful! Certificates expire in %v days", daysLeft)
	}

	config.Client.PrivatePEMPath = config.ClientCfgPath + hostname + ".key"
	config.Client.CertPath = config.ClientCfgPath + hostname + ".crt"
	config.Client.CAPath = config.ClientCfgPath + "ca.crt"
	config.Client.Server = server

	// assuming everything went well here, so we save the request
	os.WriteFile(config.Client.CertPath, []byte(identificationResponse.Certificate), 0644)
	os.WriteFile(config.Client.CAPath, []byte(identificationResponse.CA), 0644)
	os.WriteFile(config.Client.PrivatePEMPath, keyPEM, 0600)

	if saveError := config.SaveClientCfg(); saveError != nil {
		return saveError
	}

	return nil

}

func handleProvisioning(flavor string) {
	// Provisioning
	cpuSubarch, subarchError := getCpuSubarch()

	if subarchError != nil {
		fmt.Errorf("Failed to find subarch, %w", subarchError)
	}

	log.Println("Found subarch:", cpuSubarch)

	// construct JSON provisioning payload
	payload := interfaces.Provision{SubArch: cpuSubarch, Flavor: flavor}
}

func handleRegistration(token string, server string, flavor string) {
	log.Printf("Registering on server: %v with profile %v", server, flavor)

	handleIdentification(token, server)
	handleProvisioning(flavor)
}
