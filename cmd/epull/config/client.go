package clientConfig

import (
	"git.fedesito.me/fedes1to/eserve/internal/config"
)

type ClientSettings struct {
	PrivatePEMPath string `json:"pem_path"`
	Server         string `json:"server"`
	CertPath       string `json:"cert_path"`
	CAPath         string `json:"ca_path"`
}

var Client ClientSettings // i cant come up with a better naming system

var ClientConfigPath string = fetchClientPath()
var clientSettingsPath string = ClientConfigPath + "settings.json"

// populates global var Client
func LoadClientSettings() error {
	return config.LoadJsonFile(clientSettingsPath, &Client)
}

// path will never fuckin change ok?
func SaveClientSettings() error {
	return config.SafeSaveJsonFile(clientSettingsPath, Client)
}

func fetchClientPath() string {
	path := "/etc/epull/"
	config.InitSettingsPath(path)
	return path
}
