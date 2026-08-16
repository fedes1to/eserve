package serverConfig

import (
	"errors"
	"log"
	"os"

	"git.fedesito.me/fedes1to/eserve/internal/config"
	"git.fedesito.me/fedes1to/eserve/internal/sysinfo"
)

type ServerSettings struct {
	ListenAddr     string `json:"listen_addr"`
	BuildThreads   int    `json:"build_threads"`
	PerUserThreads int    `json:"per_user_threads"`
	ChrootBase     string `json:"chroot_base"`
	StagePath      string `json:"stage_base"`
	TlsCertPath    string `json:"tls_cert_path"`
	TlsKeyPath     string `json:"tls_key_path"`
}

var (
	Settings           ServerSettings // i cant come up with a better naming system
	ServerConfigPath   string         = config.InitConfigPath(config.ServerConfigPath)
	serverSettingsPath string         = ServerConfigPath + "/settings.json"
	ServerArch         string
	ServerLibc         string
)

func LoadServerSysinfo() error {
	arch, libc, gccError := sysinfo.GetGccMachine()
	if gccError != nil {
		return gccError
	}
	ServerArch, ServerLibc = arch, libc
	return nil
}

// populates global var Server
func LoadServerSettings() error {
	return config.LoadJsonFile(serverSettingsPath, &Settings)
}

func InitializeServerSettings() error {
	if _, statError := os.Stat(serverSettingsPath); statError != nil {
		if errors.Is(statError, os.ErrNotExist) {
			log.Println("Generating default server config")
			Settings.BuildThreads = 0
			Settings.PerUserThreads = 0
			Settings.ChrootBase = "/srv/build"
			Settings.ListenAddr = "127.0.0.1:8080"
			Settings.TlsCertPath = ServerConfigPath + "/server.crt"
			Settings.TlsKeyPath = ServerConfigPath + "/server.key"
			Settings.StagePath = ServerConfigPath + "/stages"
			return config.SafeSaveJsonFile(serverSettingsPath, Settings)
		}
		return statError
	}

	return LoadServerSettings()
}
