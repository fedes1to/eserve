package main

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"log"
	"net/http"
	"strings"
	"time"

	serverConfig "git.fedesito.me/fedes1to/eserve/cmd/eserved/config"
	"git.fedesito.me/fedes1to/eserve/internal/config"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

// TODO: fail after ban / rate limit
func postIdentity(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !config.IsTokenAvailable(token) {
		log.Printf("%v Attempted login with invalid token\n", clientIP(r))
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	var identificationRequest protocol.IdentificationRequest
	decodeError := json.NewDecoder(r.Body).Decode(&identificationRequest)
	if decodeError != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	block, _ := pem.Decode([]byte(identificationRequest.Csr))
	if block == nil {
		http.Error(w, "invalid csr", http.StatusBadRequest)
		return
	}

	csr, csrError := x509.ParseCertificateRequest(block.Bytes)
	if csrError != nil {
		http.Error(w, "cant parse csr", http.StatusBadRequest)
		return
	}

	if !config.ValidCN(token, csr.Subject.CommonName) {
		http.Error(w, "invalid cn or mismatch", http.StatusBadRequest)
		return
	}

	template := &x509.Certificate{
		SerialNumber: serverConfig.RandomSerial(),
		Subject:      pkix.Name{CommonName: csr.Subject.CommonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(1, 0, 0), // 1 year
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certificate, certificateError := x509.CreateCertificate(rand.Reader, template,
		serverConfig.CaCertificate, csr.PublicKey, serverConfig.CaKey)

	if certificateError != nil {
		http.Error(w, "signing cert failed", http.StatusInternalServerError)
		return
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate})

	response := protocol.IdentificationResponse{
		Certificate: string(certPEM),
		CN:          csr.Subject.CommonName,
		ValidUntil:  template.NotAfter.UTC().Format(time.RFC3339),
	}

	if tokenError := config.UseToken(token, csr.Subject.CommonName); tokenError != nil {
		http.Error(w, "couldn't use token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if encodeError := json.NewEncoder(w).Encode(response); encodeError != nil {
		http.Error(w, "couldn't encode response", http.StatusInternalServerError)
		return
	}
}

func postProvision(w http.ResponseWriter, r *http.Request) {
	// gotta make this method i guess
}
