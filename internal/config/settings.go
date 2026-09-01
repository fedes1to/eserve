package config

import (
	"log"
	"os"
	"path/filepath"
)

const (
	ServerConfigPath = "/etc/eserved"
	ClientConfigPath = "/etc/epull"
)

func InitConfigPath(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		if os.MkdirAll(path, 0755) != nil {
			log.Fatalln("Can't create config folder, is the process root?")
		}
	} else if !info.IsDir() {
		log.Fatalln("Invalid config path, please check", path)
	}
	return path
}

// a flavor's portage config: a plain dir of portage config files
func FlavorConfigDir(flavor string) string {
	return filepath.Join(ServerConfigPath, "flavors", flavor)
}
