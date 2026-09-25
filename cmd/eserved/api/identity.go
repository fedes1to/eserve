package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/chroot"
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
	if !decodeJSONBody(w, r, &identificationRequest, "identificationRequest") {
		return
	}
	if !chroot.ValidFlavor(identificationRequest.Flavor) {
		http.Error(w, "invalid flavor", http.StatusBadRequest)
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
	if err := csr.CheckSignature(); err != nil {
		http.Error(w, "csr signature invalid", http.StatusBadRequest)
		return
	}

	cn := csr.Subject.CommonName
	if !storage.ValidCN(token, cn) {
		http.Error(w, "invalid cn or mismatch", http.StatusBadRequest)
		return
	}

	template := &x509.Certificate{
		SerialNumber: storage.RandomSerial(),
		Subject:      pkix.Name{CommonName: cn},
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

	fingerprint := sha256.Sum256(certificate)
	// the token is consumed in the same lock as the machine upsert, so a token
	// can't be spent twice and an unbound one can't take over a registered cn
	if err := storage.EnrollMachine(token, cn, identificationRequest.Flavor, hex.EncodeToString(fingerprint[:])); err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, storage.ErrTokenUnknown), errors.Is(err, storage.ErrTokenUsed):
			status = http.StatusUnauthorized
		case errors.Is(err, storage.ErrTokenCN), errors.Is(err, storage.ErrTokenFlavor):
			status = http.StatusBadRequest
		case errors.Is(err, storage.ErrMachineTaken):
			status = http.StatusConflict
		}
		if status == http.StatusInternalServerError {
			log.Printf("%v: racc couldn't enroll machine %v: %v\n", ClientIP(r), cn, err)
		}
		http.Error(w, err.Error(), status)
		return
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate})
	response := protocol.IdentificationResponse{
		Certificate: string(certPEM),
		CN:          cn,
		ValidUntil:  template.NotAfter.UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "couldn't encode response", http.StatusInternalServerError)
	}
}
