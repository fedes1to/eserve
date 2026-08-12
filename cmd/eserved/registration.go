package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"git.fedesito.me/fedesito/eserve/internal/protocol"
	"net/http"
	"time"
)

func postIdentity(w http.ResponseWriter, r *http.Request) {
	var identificationRequest protocol.IdentificationRequest
	decodeError := json.NewDecoder(r.Body).Decode(&identificationRequest)
	if decodeError != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
	}

	block, blockError := pem.Decode([]byte(identificationRequest.Csr))
	if blockError != nil {
		http.Error(w, "invalid csr", http.StatusBadRequest)
	}

	csr, csrError := x509.ParseCertificateRequest(block.Bytes)
	if csrError != nil {
		http.Error(w, "cant parse csr", http.StatusBadRequest)
	}

	

}
