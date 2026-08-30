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

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
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
	if _, err := os.Stat(caCertificatePath); errors.Is(err, os.ErrNotExist) {
		if err := generateCa(); err != nil {
			return err
		}
		log.Println("created new CA certificate at", caCertificatePath)
	}

	var err error
	CaCertificatePEM, err = os.ReadFile(caCertificatePath)
	if err != nil {
		return err
	}
	pemBlock, _ := pem.Decode(CaCertificatePEM)
	if pemBlock == nil {
		return fmt.Errorf("no CA certificate parsed from %s", caCertificatePath)
	}

	CaCertificate, err = x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return err
	}

	keyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return err
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("no CA key parsed from %s", caKeyPath)
	}
	CaKey, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return err
	}

	CaPool = x509.NewCertPool()
	if !CaPool.AppendCertsFromPEM(CaCertificatePEM) {
		return fmt.Errorf("no CA certificates parsed from ca.crt")
	}

	return nil
}

// TODO: distributed signing using intermediate certs
func generateCa() error {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
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

	certificate, err := x509.CreateCertificate(rand.Reader, template,
		template, publicKey, privateKey)
	if err != nil {
		return err
	}

	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate})
	if err := os.WriteFile(caCertificatePath, certificatePEM, 0644); err != nil {
		return err
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return os.WriteFile(caKeyPath, keyPEM, 0600)
}
