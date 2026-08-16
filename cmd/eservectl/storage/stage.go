package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"git.fedesito.me/fedes1to/eserve/cmd/eservectl/download"
	serverConfig "git.fedesito.me/fedes1to/eserve/cmd/eserved/config"
)

// https://transloadit.com/devtips/verify-file-integrity-with-go-and-sha256/
func hashFileSHA256(filePath string) (string, error) {
	file, openError := os.Open(filePath)
	if openError != nil {
		return "", fmt.Errorf("failed to open file: %w", openError)
	}
	defer file.Close()

	hash := sha256.New()

	if _, copyError := io.Copy(hash, file); copyError != nil {
		return "", fmt.Errorf("failed to copy file content to hash: %w", copyError)
	}

	hashInBytes := hash.Sum(nil)
	return hex.EncodeToString(hashInBytes[:]), nil
}

func DownloadStage(url string) (string, error) {
	stagePath, pathError := GetStagePath()
	if pathError != nil {
		return "", pathError
	}

	return download.DownloadFile(stagePath, url), nil
}

func InstallStage(path string) error {
	sourceFile, openError := os.Open(path)
	if openError != nil {
		return openError
	}
	defer sourceFile.Close()

	stagePath, pathError := GetStagePath()
	if pathError != nil {
		return pathError
	}
	destinationFile, createError := os.Create(stagePath + sourceFile.Name())
	if createError != nil {
		return createError
	}
	defer destinationFile.Close()

	_, copyError := io.Copy(destinationFile, sourceFile)
	if copyError != nil {
		return copyError
	}

	return nil
}

func GetStagePath() (string, error) {
	if loadError := serverConfig.LoadServerSettings(); loadError != nil {
		return "", loadError
	}

	return serverConfig.Settings.StagePath, nil
}
