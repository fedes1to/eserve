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
	f, err := os.Open(stagePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fns, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	return fns, nil
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
