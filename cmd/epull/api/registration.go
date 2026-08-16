package api

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/epull/clientConfig"
	"git.fedesito.me/fedes1to/eserve/internal/cli"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
	"git.fedesito.me/fedes1to/eserve/internal/sysinfo"
)

func postIdentification(token string, server string, insecure bool) error {
	// making the certs and CSR
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	hostname, _ := os.Hostname()
	csrTemplate := &x509.CertificateRequest{
		// use hostname since its good enough
		Subject: pkix.Name{CommonName: hostname},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, csrTemplate, privKey)
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	if err != nil {
		return err
	}

	keyDER, _ := x509.MarshalPKCS8PrivateKey(privKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	// request to /api/v1/identity
	var client *http.Client
	if insecure {
		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
					MinVersion:         tls.VersionTLS13,
				},
			},
		}
	} else {
		client = http.DefaultClient
	}
	payload := protocol.IdentificationRequest{Csr: string(csrPEM)}
	body, _ := json.Marshal(payload)

	request, _ := http.NewRequest("POST", server+"/api/v1/identity", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	var identificationResponse protocol.IdentificationResponse
	if err := sendRequest(request, &identificationResponse, client); err != nil {
		return err
	}

	// checks to be nice n shit
	if identificationResponse.CN != hostname {
		return fmt.Errorf(
			"Hostname mismatch! expected %v, received %v\n",
			hostname,
			identificationResponse.CN)
	}

	validUntil, err := time.Parse(time.RFC3339, identificationResponse.ValidUntil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Couldn't parse valid_until field,", err)
	} else {
		daysLeft := time.Until(validUntil).Hours() / 24
		if daysLeft < 30 {
			log.Printf("Careful! Certificates expire in %v days\n", daysLeft)
		}
	}

	clientConfig.Settings.PrivatePEMPath = filepath.Join(clientConfig.ClientConfigPath, hostname+".key")
	clientConfig.Settings.CertPath = filepath.Join(clientConfig.ClientConfigPath, hostname+".crt")
	clientConfig.Settings.Server = server

	// assuming everything went well here, so we save the request
	if err := os.WriteFile(clientConfig.Settings.CertPath, []byte(identificationResponse.Certificate), 0644); err != nil {
		return fmt.Errorf("failed to save certificate: %w", err)
	}
	if err := os.WriteFile(clientConfig.Settings.PrivatePEMPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	if err := clientConfig.SaveClientSettings(); err != nil {
		return err
	}

	return nil
}

func sendRequest[T any](request *http.Request, into *T, httpClient *http.Client) error {
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("Invalid response, got code %v without body due to: %w",
				response.StatusCode, err)
		}
		return fmt.Errorf("Invalid response, got code %v with body:\n%v",
			response.StatusCode, string(bodyBytes))
	}

	// decode json to struct
	err = json.NewDecoder(response.Body).Decode(into)
	if err != nil {
		return err
	}

	return nil
}

func sendMtlsRequest[T any](subUrl string, payload any, into *T) error {
	body, _ := json.Marshal(payload)
	request, _ := http.NewRequest("POST", clientConfig.Settings.Server+subUrl, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	return sendRequest(request, into, mtlsClient)
}

func postProvisioning(flavor, stage string) error {
	// Provisioning
	cpuSubarch, err := sysinfo.GetCpuSubarch()
	if err != nil {
		return err
	}

	gccMachine, err := sysinfo.GetGccMachine()
	if err != nil {
		return err
	}

	profile, err := sysinfo.GetPortageProfile()
	if err != nil {
		return err
	}

	log.Printf("Found profile %v with subarch %v\n", profile, cpuSubarch)

	// construct JSON provisioning payload
	payload := protocol.ProvisionRequest{GccMachine: gccMachine, Subarch: cpuSubarch, Profile: profile, Stagefile: stage, Flavor: flavor}
	var provisionResponse protocol.ProvisionResponse
	err = sendMtlsRequest("/api/v1/provision", payload, &provisionResponse)
	if err != nil {
		return err
	}

	return nil
}

func HandleProvision(flavor, stage string) (error, int) {
	log.Printf("Provisioning with flavor %v", flavor)

	return cli.MustRegister([]cli.InitStep{
		{Name: "provisioning", Function: func() error { return postProvisioning(flavor, stage) }},
	})
}

func HandleRegistration(token, server, flavor string, insecure bool) (error, int) {
	log.Printf("Registering on server: %v with flavor %v", server, flavor)

	err, exitCode := cli.MustRegister([]cli.InitStep{
		{Name: "identity", Function: func() error { return postIdentification(token, server, insecure) }},
		{Name: "mtls client", Function: func() error { return InitializeMtlsClient(insecure) }},
	})
	if err != nil {
		return err, exitCode
	}

	stage, err := AskStagefile()
	if err != nil {
		return err, 1
	}
	return cli.MustRegister([]cli.InitStep{
		{Name: "provision", Function: func() error { return postProvisioning(flavor, stage) }}})

}
