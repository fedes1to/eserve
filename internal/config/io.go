package config

import (
	"encoding/json"
	"errors"
	"io"
	"os"
)

func SafeSaveJsonFile(path string, from any) error {
	tmpPath := path + ".tmp"
	jsonFile, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer jsonFile.Close()
	defer os.Remove(tmpPath)

	encoder := json.NewEncoder(jsonFile)
	encoder.SetIndent("", "  ") // make it tolerable to see
	if err := encoder.Encode(from); err != nil {
		return err
	}

	if err := jsonFile.Sync(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	// the rename carries the tmp file's mode, but an existing file keeps its own
	return os.Chmod(path, 0o600)
}

func LoadJsonFile[T any](path string, into *T) error {
	jsonFile, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // its just an empty file
		}
		return err
	}
	defer jsonFile.Close()

	decoder := json.NewDecoder(jsonFile)
	if err := decoder.Decode(into); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
