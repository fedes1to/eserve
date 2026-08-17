package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"log"
	"net/http"
	"strings"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

func PostIdentity(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !storage.IsTokenAvailable(token) {
		log.Printf("%v Attempted identification with invalid token\n", ClientIP(r))
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	var identificationRequest protocol.IdentificationRequest
	if err := json.NewDecoder(r.Body).Decode(&identificationRequest); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	block, _ := pem.Decode([]byte(identificationRequest.Csr))
	if block == nil {
		http.Error(w, "invalid csr", http.StatusBadRequest)
		return
	}

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		http.Error(w, "cant parse csr", http.StatusBadRequest)
		return
	}

	if !storage.ValidCN(token, csr.Subject.CommonName) {
		http.Error(w, "invalid cn or mismatch", http.StatusBadRequest)
		return
	}

	template := &x509.Certificate{
		SerialNumber: storage.RandomSerial(),
		Subject:      pkix.Name{CommonName: csr.Subject.CommonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(1, 0, 0), // 1 year
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certificate, err := x509.CreateCertificate(rand.Reader, template,
		storage.CaCertificate, csr.PublicKey, storage.CaKey)

	if err != nil {
		http.Error(w, "signing cert failed", http.StatusInternalServerError)
		return
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate})

	response := protocol.IdentificationResponse{
		Certificate: string(certPEM),
		CN:          csr.Subject.CommonName,
		ValidUntil:  template.NotAfter.UTC().Format(time.RFC3339),
	}

	if err := storage.UseToken(token, csr.Subject.CommonName); err != nil {
		http.Error(w, "couldn't use token", http.StatusInternalServerError)
		return
	}

	fingerprint := sha256.Sum256(certificate)
	if err := storage.UpsertMachine(csr.Subject.CommonName, hex.EncodeToString(fingerprint[:])); err != nil {
		http.Error(w, "couldn't upsert machine", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "couldn't encode response", http.StatusInternalServerError)
		return
	}
}

func PostProvision(w http.ResponseWriter, r *http.Request) {
	var provisionRequest protocol.ProvisionRequest
	if err := json.NewDecoder(r.Body).Decode(&provisionRequest); err != nil {
		http.Error(w, "couldn't decode provisionRequest", http.StatusBadRequest)
		return
	}

	identity := r.Context().Value(CtxKeyIdentity).(ClientIdentity)
	if err := storage.ProvisionMachine(
		identity.CN, provisionRequest.Subarch, provisionRequest.GccMachine, provisionRequest.Profile, provisionRequest.Flavor); err != nil {
		log.Printf("%v: %v\n", ClientIP(r), err)
		http.Error(w, "couldn't provision machine", http.StatusInternalServerError)
		return
	}

	

}
