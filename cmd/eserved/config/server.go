package serverConfig

import (
	"git.fedesito.me/fedes1to/eserve/internal/config"
)

var path = config.InitConfigPath(config.ServerConfigPath)

type ServerSettings struct {
	ListenAddr     string `json:"listen_addr"`
	BuildThreads   string `json:"build_threads"`
	PerUserThreads string `json:"per_user_threads"`
	ChrootBase     string `json:"chroot_base"`
}

var Settings ServerSettings // i cant come up with a better naming system

var ServerConfigPath string = config.InitConfigPath(config.ServerConfigPath)
var serverSettingsPath string = ServerConfigPath + "settings.json"

// populates global var Server
func LoadServerSettings() error {
	return config.LoadJsonFile(serverSettingsPath, &Settings)
}

// path will never fuckin change ok?
func SaveServerSettings() error {
	return config.SafeSaveJsonFile(serverSettingsPath, Settings)
}
