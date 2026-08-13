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

var Settings ClientSettings // i cant come up with a better naming system

var ClientConfigPath string = config.InitConfigPath(config.ClientConfigPath)
var clientSettingsPath string = ClientConfigPath + "settings.json"

// populates global var Client
func LoadClientSettings() error {
	return config.LoadJsonFile(clientSettingsPath, &Settings)
}

// path will never fuckin change ok?
func SaveClientSettings() error {
	return config.SafeSaveJsonFile(clientSettingsPath, Settings)
}
