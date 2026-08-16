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
	if _, err := os.Stat(serverConfig.Settings.TlsCertPath); errors.Is(err, os.ErrNotExist) {
		log.Println("no server cert found, generating self-signed cert")
		if err := generateSelfSigned(); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("checking server cert: %w", err)
	}

	cert, err := tls.LoadX509KeyPair(serverConfig.Settings.TlsCertPath, serverConfig.Settings.TlsKeyPath)
	if err != nil {
		return fmt.Errorf("loading server cert: %w", err)
	}
	TlsCertificate = cert
	return nil
}

func generateSelfSigned() error {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	template := &x509.Certificate{
		SerialNumber: RandomSerial(),
		Subject:      pkix.Name{CommonName: "eserver"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(1, 0, 0), // 1 year
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certificate, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		return err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate})
	if err := os.WriteFile(serverConfig.Settings.TlsCertPath, certPEM, 0644); err != nil {
		return err
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return os.WriteFile(serverConfig.Settings.TlsKeyPath, keyPEM, 0600)
}
