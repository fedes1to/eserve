package serverConfig

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"os"
	"time"
)

var TlsCertificate tls.Certificate

func LoadTlsCertificates() error {
	cert, certError := tls.LoadX509KeyPair(Settings.TlsCertPath, Settings.TlsKeyPath)
	if certError == nil {
		TlsCertificate = cert
		return nil
	}
	if !errors.Is(certError, os.ErrNotExist) {
		return fmt.Errorf("Failed loading server cert: %w", certError)
	}

	log.Println("no server cert found, generating self-signed cert")
	if generateError := generateSelfSigned(); generateError != nil {
		return generateError
	}
	return nil
}

func generateSelfSigned() error {
	publicKey, privateKey, keyError := ed25519.GenerateKey(rand.Reader)
	if keyError != nil {
		return keyError
	}

	template := &x509.Certificate{
		SerialNumber: RandomSerial(),
		Subject:      pkix.Name{CommonName: "eserver"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(1, 0, 0), // 1 year
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certificate, certificateError := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if certificateError != nil {
		return certificateError
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate})
	if writeError := os.WriteFile(Settings.TlsCertPath, certPEM, 0644); writeError != nil {
		return writeError
	}

	keyDER, keyError := x509.MarshalPKCS8PrivateKey(privateKey)
	if keyError != nil {
		return keyError
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return os.WriteFile(Settings.TlsKeyPath, keyPEM, 0600)
}
