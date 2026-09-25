package config

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
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

// state files written before SafeSaveJsonFile got its 0600 keep their old mode
// until something rewrites them, so bring them back in line
func TightenMode(path string, mode os.FileMode) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Mode().Perm() == mode.Perm() {
		return nil
	}
	return os.Chmod(path, mode)
}

// every directory under dir gets dirMode, every regular file fileMode
func TightenTree(dir string, dirMode, fileMode os.FileMode) error {
	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			return TightenMode(path, dirMode)
		}
		if entry.Type().IsRegular() {
			return TightenMode(path, fileMode)
		}
		return nil
	})
}
