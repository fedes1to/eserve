package storage

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"time"

	serverConfig "git.fedesito.me/fedes1to/eserve/cmd/eserved/config"
)

var (
	CaCertificate     *x509.Certificate
	CaKey             any
	CaCertificatePEM  []byte
	caCertificatePath string = filepath.Join(serverConfig.ServerConfigPath, "ca.crt")
	caKeyPath         string = filepath.Join(serverConfig.ServerConfigPath, "ca.key")
	CaPool            *x509.CertPool
)

func RandomSerial() *big.Int {
	b := make([]byte, 16) // 128 bits
	rand.Read(b)
	b[0] &= 0x7f // always should be positive
	return new(big.Int).SetBytes(b)
}

func LoadCaCertificate() error {
	if _, certError := os.Stat(caCertificatePath); errors.Is(certError, os.ErrNotExist) {
		if err := generateCa(); err != nil {
			return certError
		}
		log.Println("created new CA certificate at", caCertificatePath)
	}

	var pemError error
	CaCertificatePEM, pemError = os.ReadFile(caCertificatePath)
	if pemError != nil {
		return pemError
	}
	pemBlock, _ := pem.Decode(CaCertificatePEM)

	var certError error
	CaCertificate, certError = x509.ParseCertificate(pemBlock.Bytes)
	if certError != nil {
		return certError
	}

	keyPEM, keyError := os.ReadFile(caKeyPath)
	if keyError != nil {
		return keyError
	}

	keyBlock, _ := pem.Decode(keyPEM)
	var caKeyError error
	CaKey, caKeyError = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if caKeyError != nil {
		return caKeyError
	}

	CaPool = x509.NewCertPool()
	if !CaPool.AppendCertsFromPEM(CaCertificatePEM) {
		return fmt.Errorf("no CA certificates parsed from ca.crt")
	}

	return nil
}

// TODO: distributed signing using intermediate certs
func generateCa() error {
	publicKey, privateKey, keyError := ed25519.GenerateKey(rand.Reader)
	if keyError != nil {
		return keyError
	}

	template := &x509.Certificate{
		SerialNumber:          RandomSerial(),
		Subject:               pkix.Name{CommonName: "eserve-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 years
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	certificate, certificateError := x509.CreateCertificate(rand.Reader, template,
		template, publicKey, privateKey)
	if certificateError != nil {
		return certificateError
	}

	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate})
	if fileError := os.WriteFile(caCertificatePath, certificatePEM, 0644); fileError != nil {
		return fileError
	}

	keyDER, pkcs8Error := x509.MarshalPKCS8PrivateKey(privateKey)
	if pkcs8Error != nil {
		return pkcs8Error
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return os.WriteFile(caKeyPath, keyPEM, 0600)
}
