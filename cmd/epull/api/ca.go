package api

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"git.fedesito.me/fedes1to/eserve/internal/config"
	"git.fedesito.me/fedes1to/eserve/internal/urls"
)

// one-time bootstrap: fetch the server CA over an unverified hop and pin it
// locally; every epull request after that verifies against it
func PinServerCa(server string) error {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13},
		},
	}

	response, err := client.Get(server + urls.CaSuburl)
	if err != nil {
		return fmt.Errorf("couldn't fetch the server CA: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("server has no CA endpoint, code %v", response.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if !x509.NewCertPool().AppendCertsFromPEM(data) {
		return fmt.Errorf("the server returned no valid CA certificate")
	}

	if err := os.MkdirAll(config.ClientConfigPath, 0700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(config.ClientConfigPath, "ca.crt"), data, 0644)
}

// pin the CA on first run so steady-state commands verify without -insecure
func pinIfMissing(server string, insecure bool) error {
	if _, err := os.Stat(filepath.Join(config.ClientConfigPath, "ca.crt")); !os.IsNotExist(err) {
		return nil
	}
	if err := PinServerCa(server); err != nil {
		if insecure {
			log.Printf("continuing without a pinned CA: %v", err)
			return nil
		}
		return err
	}
	return nil
}
