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
	"net"
	"os"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
)

var TlsCertificate tls.Certificate

func LoadTlsCertificates() error {
	if _, err := os.Stat(serverConfig.Settings.TlsCertPath); errors.Is(err, os.ErrNotExist) {
		log.Println("no server cert found, generating one signed by the eserved CA")
		if err := generateServerCert(); err != nil {
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

// signed by the eserved CA so plain https clients (like portage) can
// verify it; SANs cover whatever address clients reach us on
func generateServerCert() error {
	if CaCertificate == nil || CaKey == nil {
		return fmt.Errorf("CA not loaded, can't sign the server cert")
	}
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
		DNSNames:     []string{"eserver"},
		IPAddresses:  serverIPs(),
	}

	certificate, err := x509.CreateCertificate(rand.Reader, template, CaCertificate, publicKey, CaKey)
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

// loopback plus every non-loopback ipv4 up on this host
func serverIPs() []net.IP {
	ips := []net.IP{net.ParseIP("127.0.0.1")}
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP)
			}
		}
	}
	return ips
}
