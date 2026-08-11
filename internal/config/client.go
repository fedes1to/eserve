package config

import (
	"encoding/json"
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

var ClientCfgPath string = fetchPath()
var clientSettingsPath string = ClientCfgPath + "settings.json"

// populates global var Client
func LoadClient() error {
	jsonFile, openError := os.Open(clientSettingsPath)
	if openError != nil {
		return openError
	}
	defer jsonFile.Close()

	decoder := json.NewDecoder(jsonFile)
	if decodeError := decoder.Decode(&Client); decodeError != nil {
		return decodeError
	}
	return nil
}

// path will never fuckin change ok?
func SaveClientCfg() error {
	jsonFile, createError := os.Create(clientSettingsPath) // truncates if it exists
	if createError != nil {
		return createError
	}
	defer jsonFile.Close()

	encoder := json.NewEncoder(jsonFile)
	if encodeError := encoder.Encode(Client); encodeError != nil {
		return encodeError
	}
	return nil
}

func fetchPath() string {
	path := "/etc/epull/" // i think its stupid to put it in user config

	info, err := os.Stat(path)
	if err != nil {
		// maybe .config doesn't exist? so we use MkdirAll
		if os.MkdirAll(path, 0644) != nil {
			log.Fatalln("Can't create config folder, is the process root?")
		}
	} else if !info.IsDir() {
		log.Fatalln("Invalid config path, please check /etc/epull")
	}

	return path
}
