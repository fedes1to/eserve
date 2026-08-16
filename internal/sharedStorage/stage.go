package sharedStorage

import (
	"os"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
)

func GetStageList() ([]string, error) {
	stagePath, pathError := GetStagePath()
	if pathError != nil {
		return nil, pathError
	}
	f, err := os.Open(stagePath)
	defer func() { _ = f.Close() }()
	if err != nil {
		return nil, err
	}
	fns, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	return fns, nil
}

func GetStagePath() (string, error) {
	if serverConfig.Settings.StagePath == "" {
		loadError := serverConfig.LoadServerSettings()
		if loadError != nil {
			return "", loadError
		}
	}

	return serverConfig.Settings.StagePath, nil
}
