package config

import (
	"log"
	"os"
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
	return loadJsonFile(clientSettingsPath, &Client)
}

// path will never fuckin change ok?
func SaveClientSettings() error {
	return safeSaveJsonFile(clientSettingsPath, Client)
}

func fetchClientPath() string {
	path := "/etc/epull/" // i think its stupid to put it in user config

	info, err := os.Stat(path)
	if err != nil {
		// maybe /etc/ doesn't exist (x doubt)? so we use MkdirAll
		if os.MkdirAll(path, 0644) != nil {
			log.Fatalln("Can't create config folder, is the process root?")
		}
	} else if !info.IsDir() {
		log.Fatalln("Invalid config path, please check", path)
	}

	return path
}
