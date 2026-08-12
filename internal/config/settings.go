package config

import (
	"log"
	"os"
)

func initSettingsPath(path string) {
	info, err := os.Stat(path)
	if err != nil {
		// maybe /etc/ doesn't exist (x doubt)? so we use MkdirAll
		if os.MkdirAll(path, 0644) != nil {
			log.Fatalln("Can't create config folder, is the process root?")
		}
	} else if !info.IsDir() {
		log.Fatalln("Invalid config path, please check", path)
	}
}
