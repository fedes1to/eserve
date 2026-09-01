package serverConfig

import (
	"errors"
	"log"
	"os"
	"path/filepath"

	"git.fedesito.me/fedes1to/eserve/internal/config"
)

type ServerSettings struct {
	ListenAddr     string `json:"listen_addr"`
	BuildThreads   int    `json:"build_threads"`    // 0 = unlimited
	PerUserThreads int    `json:"per_user_threads"` // 0 = unlimited
	ChrootBase     string `json:"chroot_base"`
	StagePath      string `json:"stage_path"`
	TlsCertPath    string `json:"tls_cert_path"`
	TlsKeyPath     string `json:"tls_key_path"`
	BaseBinhostURL string `json:"base_binhost_url"`
	RepoBase       string `json:"repo_base"`
}

var (
	Settings           ServerSettings // i cant come up with a better naming system
	ServerConfigPath   string         = config.InitConfigPath(config.ServerConfigPath)
	serverSettingsPath string         = filepath.Join(ServerConfigPath, "settings.json")
)

func LoadServerSettings() error {
	return config.LoadJsonFile(serverSettingsPath, &Settings)
}

func InitializeServerSettings() error {
	_, err := os.Stat(serverSettingsPath)
	if err == nil {
		return LoadServerSettings()
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	log.Println("Generating default server config")
	const listenAddr = "127.0.0.1:8080"
	Settings = ServerSettings{
		ListenAddr:     listenAddr,
		ChrootBase:     "/srv/build",
		RepoBase:       "/srv/pkgs",
		BaseBinhostURL: "https://" + listenAddr + "/pkgs",
		StagePath:      filepath.Join(ServerConfigPath, "stages"),
		TlsCertPath:    filepath.Join(ServerConfigPath, "server.crt"),
		TlsKeyPath:     filepath.Join(ServerConfigPath, "server.key"),
	}
	return config.SafeSaveJsonFile(serverSettingsPath, Settings)
}
