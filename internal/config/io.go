package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

func SafeSaveJsonFile(path string, from any) error {
	backupPath := path + ".old"
	oldJson, openErr := os.Open(path)
	backupJson, backupErr := os.Create(backupPath)

	if openErr != nil && !errors.Is(openErr, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "Couldn't open old JSON for backup,", openErr)
	} else if backupErr != nil {
		fmt.Fprintln(os.Stderr, "Couldn't access target backup JSON,", backupErr)
	} else if openErr == nil {
		_, copyErr := io.Copy(backupJson, oldJson)
		if copyErr != nil {
			fmt.Fprintln(os.Stderr, "Couldn't copy backup JSON,", copyErr)
		}
		defer oldJson.Close()
		defer backupJson.Close()
	}

	// truncates if it exists, thats why we made a .old
	jsonFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	encoder := json.NewEncoder(jsonFile)
	encoder.SetIndent("", "  ") // make it tolerable to see
	if err := encoder.Encode(from); err != nil {
		return err
	}

	return nil
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
	if err := decoder.Decode(into); err != nil {
		return err
	}
	return nil
}
