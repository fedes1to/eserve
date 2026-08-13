package config

import (
	"log"
	"os"
)

// hardcoding go brrr i guess
var (
	ServerConfigPath = "/etc/eserved/"
	ClientConfigPath = "/etc/epull/"
)

func InitConfigPath(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		// maybe /etc/ doesn't exist (x doubt)? so we use MkdirAll
		if os.MkdirAll(path, 0644) != nil {
			log.Fatalln("Can't create config folder, is the process root?")
		}
	} else if !info.IsDir() {
		log.Fatalln("Invalid config path, please check", path)
	}
	return path // can be safely ignored, its just for convenience
}
