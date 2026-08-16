package storage

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

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
)

var TlsCertificate tls.Certificate

func LoadTlsCertificates() error {
	if _, statError := os.Stat(serverConfig.Settings.TlsCertPath); errors.Is(statError, os.ErrNotExist) {
		log.Println("no server cert found, generating self-signed cert")
		if generateError := generateSelfSigned(); generateError != nil {
			return generateError
		}
	} else if statError != nil {
		return fmt.Errorf("checking server cert: %w", statError)
	}

	cert, certError := tls.LoadX509KeyPair(serverConfig.Settings.TlsCertPath, serverConfig.Settings.TlsKeyPath)
	if certError != nil {
		return fmt.Errorf("loading server cert: %w", certError)
	}
	TlsCertificate = cert
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
	if writeError := os.WriteFile(serverConfig.Settings.TlsCertPath, certPEM, 0644); writeError != nil {
		return writeError
	}

	keyDER, keyError := x509.MarshalPKCS8PrivateKey(privateKey)
	if keyError != nil {
		return keyError
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return os.WriteFile(serverConfig.Settings.TlsKeyPath, keyPEM, 0600)
}
