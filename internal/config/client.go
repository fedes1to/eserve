package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type ClientSettings struct {
	PrivatePEMPath string `json:"pem_path"`
	Server         string `json:"server"`
}

var Client ClientSettings // i cant come up with a better naming system

var clientCfgPath string = fetchPath()
var clientSettingsPath string = clientCfgPath + "settings.json"

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
	path, err := os.UserConfigDir()
	if err != nil {
		fmt.Errorf("No user config dir found, %w", err)
	}

	path += "/epull/"
	info, err := os.Stat(path)
	if err != nil {
		// maybe .config doesn't exist? so we use MkdirAll
		os.MkdirAll(path)
	} else if !info.IsDir() {
		fmt.Errorf("Invalid config path, please check %v", path)
	}

	return path
}
