package config

import (
	"encoding/json"
	"errors"
	"io"
	"os"
)

func SafeSaveJsonFile(path string, from any) error {
	tmpPath := path + ".tmp"
	jsonFile, err := os.Create(tmpPath)
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

	return os.Rename(tmpPath, path)
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
