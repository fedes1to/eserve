package config

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
	path := "/etc/epull/"
	initSettingsPath(path)
	return path
}
