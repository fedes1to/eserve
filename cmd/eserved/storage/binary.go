package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/config"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

func binaryDir() string {
	return filepath.Join(serverConfig.Settings.RepoBase, "binaries")
}

func binaryPath(name, arch string) string {
	return filepath.Join(binaryDir(), arch, name)
}

func binaryManifestPath(name, arch string) string {
	return binaryPath(name, arch) + ".json"
}

func validateBinaryName(name string) error {
	if name == "" || len(name) > 64 || strings.ContainsAny(name, "/\\ ") {
		return fmt.Errorf("invalid binary name %q: no slashes or spaces", name)
	}
	return nil
}

func validateArch(arch string) error {
	// the arch ends up in the path, same treatment
	if arch == "" || len(arch) > 64 || strings.ContainsAny(arch, "/\\ ") {
		return fmt.Errorf("invalid arch %q: no slashes or spaces", arch)
	}
	return nil
}

func UploadBinary(manifest protocol.BinaryManifest, file io.Reader) error {
	if err := validateBinaryName(manifest.Name); err != nil {
		return err
	}
	if err := validateArch(manifest.Arch); err != nil {
		return err
	}
	if manifest.SHA256 == "" {
		return fmt.Errorf("the manifest needs a sha256")
	}

	archDir := filepath.Join(binaryDir(), manifest.Arch)
	if err := os.MkdirAll(archDir, 0o755); err != nil {
		return err
	}

	tmp := binaryPath(manifest.Name, manifest.Arch) + ".new"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(out, hasher), io.LimitReader(file, protocol.MaxBinarySize+1))
	out.Close()
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if written > protocol.MaxBinarySize {
		os.Remove(tmp)
		return fmt.Errorf("binary is bigger than the %d byte limit", protocol.MaxBinarySize)
	}

	sum := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(sum, manifest.SHA256) {
		os.Remove(tmp)
		return fmt.Errorf("sha256 mismatch: got %s, claimed %s", sum, manifest.SHA256)
	}

	manifest.Size = written
	if err := os.Rename(tmp, binaryPath(manifest.Name, manifest.Arch)); err != nil {
		os.Remove(tmp)
		return err
	}
	os.Chmod(binaryPath(manifest.Name, manifest.Arch), 0o755)

	return config.SafeSaveJsonFile(binaryManifestPath(manifest.Name, manifest.Arch), manifest)
}

func GetBinary(name, arch string) (path string, manifest protocol.BinaryManifest, err error) {
	if err := validateBinaryName(name); err != nil {
		return "", manifest, err
	}
	if err := validateArch(arch); err != nil {
		return "", manifest, err
	}

	path = binaryPath(name, arch)
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		return "", manifest, fmt.Errorf("no %s build of %s", arch, name)
	}

	if err := config.LoadJsonFile(binaryManifestPath(name, arch), &manifest); err != nil {
		return "", manifest, fmt.Errorf("no manifest for %s/%s", arch, name)
	}
	return path, manifest, nil
}

func ListBinaries() ([]protocol.BinaryManifest, error) {
	binaries := []protocol.BinaryManifest{}

	archDirs, err := os.ReadDir(binaryDir())
	if err != nil {
		if os.IsNotExist(err) {
			return binaries, nil
		}
		return nil, err
	}

	for _, archDir := range archDirs {
		if !archDir.IsDir() {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(binaryDir(), archDir.Name()))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			var manifest protocol.BinaryManifest
			if err := config.LoadJsonFile(binaryManifestPath(entry.Name(), archDir.Name()), &manifest); err != nil {
				continue // binary without a manifest, skip it
			}
			binaries = append(binaries, manifest)
		}
	}

	sort.Slice(binaries, func(i, j int) bool {
		if binaries[i].Arch != binaries[j].Arch {
			return binaries[i].Arch < binaries[j].Arch
		}
		return binaries[i].Name < binaries[j].Name
	})
	return binaries, nil
}
