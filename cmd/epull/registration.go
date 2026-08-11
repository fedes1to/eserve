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

	"git.fedesito.me/fedesito/eserve/internal/protocol"
)

func handleIdentification(token string, server string) {
	// making the certs and CSR
	_, privKey, keyError := ed25519.GenerateKey()
	if keyError != nil {
		fmt.Errorf("Couldn't generate ed25519 key, %w", keyError)
	}

	hostname, _ := os.Hostname()
	csrTemplate := &x509.CertificateRequest{
		// use hostname since its good enough
		Subject: pkix.Name{CommonName: hostname},
	}
	csrDER, csrError := x509.CreateCertificateRequest(rand.Reader, csrTemplate, privKey)
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	if csrError != nil {
		fmt.Errorf("Couldn't create certificate request, %w", csrError)
	}

	keyDER, _ := x509.MarshalPKCS8PrivateKey(privKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	// request to /api/v1/identity
	payload := protocols.IdentificationRequest{Csr: string(csrPEM)}
	body, _ := json.Marshal(payload)

	request, _ := http.NewRequest("POST", server+"/api/v1/identity", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	response, responseError := http.DefaultClient.Do(request)
	if responseError != nil {
		fmt.Errorf("Identity request failed, %w", responseError)
	}

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
