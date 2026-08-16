package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"git.fedesito.me/fedes1to/eserve/cmd/eservectl/download"
	"git.fedesito.me/fedes1to/eserve/internal/sharedStorage"
)

// https://transloadit.com/devtips/verify-file-integrity-with-go-and-sha256/
func hashFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()

	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to copy file content to hash: %w", err)
	}

	hashInBytes := hash.Sum(nil)
	return hex.EncodeToString(hashInBytes[:]), nil
}

func GetStageHash(filename string) (string, error) {
	stagePath, err := sharedStorage.GetStagePath()
	if err != nil {
		return "", err
	}

	return hashFileSHA256(filepath.Join(stagePath, filename))

}

func DownloadStage(url string) (string, error) {
	stagePath, err := sharedStorage.GetStagePath()
	if err != nil {
		return "", err
	}

	fileName, err := download.DownloadFile(url, stagePath)
	if err != nil {
		return "", err
	}

	return fileName, nil
}

func InstallStage(path string) error {
	sourceFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	stagePath, err := sharedStorage.GetStagePath()
	if err != nil {
		return err
	}
	destinationFile, err := os.Create(filepath.Join(stagePath, filepath.Base(path)))
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return err
	}

	return nil
}
