package main

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"strings"

	"git.fedesito.me/fedesito/eserve/internal/config"
	"git.fedesito.me/fedesito/eserve/internal/protocol"
)

// TODO: fail after ban / rate limit
func postIdentity(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !config.IsTokenAvailable(token) {
		http.Error(w, "bad token", http.StatusBadRequest)
		return
	}

	var identificationRequest protocol.IdentificationRequest
	decodeError := json.NewDecoder(r.Body).Decode(&identificationRequest)
	if decodeError != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	block, blockError := pem.Decode([]byte(identificationRequest.Csr))
	if blockError != nil {
		http.Error(w, "invalid csr", http.StatusBadRequest)
		return
	}

	csr, csrError := x509.ParseCertificateRequest(block.Bytes)
	if csrError != nil {
		http.Error(w, "cant parse csr", http.StatusBadRequest)
		return
	}

}
