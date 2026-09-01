package clientConfig

import (
	"path/filepath"

	"git.fedesito.me/fedes1to/eserve/internal/config"
)

type ClientSettings struct {
	PrivatePEMPath string `json:"pem_path"`
	Server         string `json:"server"`
	CertPath       string `json:"cert_path"`
}

var Settings ClientSettings // i cant come up with a better naming system

var ClientConfigPath string = config.InitConfigPath(config.ClientConfigPath)
var clientSettingsPath string = filepath.Join(ClientConfigPath, "settings.json")

func LoadClientSettings() error {
	return config.LoadJsonFile(clientSettingsPath, &Settings)
}

func SaveClientSettings() error {
	return config.SafeSaveJsonFile(clientSettingsPath, Settings)
}
