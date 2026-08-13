package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

func SafeSaveJsonFile(path string, from any) error {
	backupPath := path + ".bak"
	oldJson, openError := os.Open(path)
	backupJson, backupError := os.Create(backupPath)

	if openError != nil && !errors.Is(openError, fs.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "Couldn't open old JSON for backup,", openError)
	} else if backupError != nil {
		fmt.Fprintln(os.Stderr, "Couldn't access target backup JSON,", backupError)
	} else if openError == nil {
		_, copyError := io.Copy(backupJson, oldJson)
		if copyError != nil {
			fmt.Fprintln(os.Stderr, "Couldn't copy backup JSON,", copyError)
		}
		defer oldJson.Close()
		defer backupJson.Close()
	}

	// truncates if it exists, thats why we made a .bak
	jsonFile, createError := os.Create(path)
	if createError != nil {
		return createError
	}
	defer jsonFile.Close()

	encoder := json.NewEncoder(jsonFile)
	if encodeError := encoder.Encode(from); encodeError != nil {
		return encodeError
	}

	return nil
}

func LoadJsonFile[T any](path string, into *T) error {
	jsonFile, openError := os.Open(path)
	if openError != nil {
		return openError
	}
	defer jsonFile.Close()

	decoder := json.NewDecoder(jsonFile)
	if decodeError := decoder.Decode(into); decodeError != nil {
		return decodeError
	}
	return nil
}
