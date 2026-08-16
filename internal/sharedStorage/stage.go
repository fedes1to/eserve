package sharedStorage

import (
	"os"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
)

func GetStageList() ([]string, error) {
	stagePath, err := GetStagePath()
	if err != nil {
		return nil, err
	}
	dir, err := os.Open(stagePath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	filenames, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	return filenames, nil
}

func GetStagePath() (string, error) {
	if serverConfig.Settings.StagePath == "" {
		err := serverConfig.LoadServerSettings()
		if err != nil {
			return "", err
		}
	}

	return serverConfig.Settings.StagePath, nil
}
