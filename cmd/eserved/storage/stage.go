package storage

import (
	"fmt"
	"os"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
)

func InitializeStageFolder() error {
	info, err := os.Stat(serverConfig.Settings.StagePath)
	if err != nil {
		if mkdirErr := os.MkdirAll(serverConfig.Settings.StagePath, 0755); mkdirErr != nil {
			return fmt.Errorf("Can't create stage folder, %w", mkdirErr)
		}
	} else if !info.IsDir() {
		return fmt.Errorf("Stage path is a file, please check %v", serverConfig.Settings.StagePath)
	}
	return nil
}
