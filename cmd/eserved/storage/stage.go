package storage

import (
	"fmt"
	"os"

	serverConfig "git.fedesito.me/fedes1to/eserve/cmd/eserved/config"
)

func InitializeStageFolder() error {
	info, err := os.Stat(serverConfig.Settings.StagePath)
	if err != nil {
		mkdirError := os.MkdirAll(serverConfig.Settings.StagePath, 0644)
		if mkdirError != nil {
			return fmt.Errorf("Can't create stage folder, %w", mkdirError)
		}
	} else if !info.IsDir() {
		return fmt.Errorf("Stage path is a file, please check %v", serverConfig.Settings.StagePath)
	}
	return nil
}
