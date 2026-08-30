package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"git.fedesito.me/fedes1to/eserve/cmd/epull/clientConfig"
	"git.fedesito.me/fedes1to/eserve/cmd/epull/storage"
	"git.fedesito.me/fedes1to/eserve/internal/cli"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
	"git.fedesito.me/fedes1to/eserve/internal/urls"
)

// stable across runs: mtimes and ownership are normalized, missing paths skipped
func SyncPortage() (fingerprint string, archive []byte, err error) {
	var archiveData bytes.Buffer
	gzipWriter := gzip.NewWriter(&archiveData)
	tarWriter := tar.NewWriter(gzipWriter)
	manifest := protocol.NewPortageManifest()

	paths := append([]string(nil), protocol.PortageSyncPaths...)
	sort.Strings(paths)
	for _, relativePath := range paths {
		if err := writePortagePath(tarWriter, manifest, relativePath); err != nil {
			return "", nil, err
		}
	}
	if err := tarWriter.Close(); err != nil {
		return "", nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return "", nil, err
	}

	return manifest.Fingerprint(), archiveData.Bytes(), nil
}

func CheckSync(fingerprint string) (inSync bool, check protocol.SyncCheckResponse, err error) {
	request, err := http.NewRequest("POST", clientConfig.Settings.Server+urls.CheckSyncSuburl, nil)
	if err != nil {
		return false, check, err
	}
	request.Header.Set("X-Portage-Fingerprint", fingerprint)

	response, err := mtlsClient.Do(request)
	if err != nil {
		return false, check, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNoContent {
		return true, check, nil
	}
	if response.StatusCode == http.StatusConflict {
		if err := json.NewDecoder(response.Body).Decode(&check); err != nil {
			return false, check, fmt.Errorf("couldn't decode sync check response: %w", err)
		}
		return false, check, nil
	}
	body, _ := io.ReadAll(response.Body)
	return false, check, fmt.Errorf("couldn't check portage config, code %v, body: %s", response.StatusCode, string(body))
}

func UploadPortage(fingerprint string, archive []byte) error {
	request, err := http.NewRequest("POST", clientConfig.Settings.Server+urls.SyncSuburl, bytes.NewReader(archive))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/gzip")
	request.Header.Set("X-Portage-Fingerprint", fingerprint)

	response, err := mtlsClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("couldn't sync portage config, code %v, body: %s", response.StatusCode, string(body))
	}
	return nil
}

func stdinIsTerminal() bool {
	stat, err := os.Stdin.Stat()
	return err == nil && stat.Mode()&os.ModeCharDevice != 0
}

func runSync(assumeYes bool) error {
	fingerprint, archive, err := SyncPortage()
	if err != nil {
		return fmt.Errorf("couldn't read portage config: %w", err)
	}

	inSync, check, err := CheckSync(fingerprint)
	if err != nil {
		return err
	}
	if inSync {
		log.Println("portage config is in sync with the server")
		return nil
	}

	if !assumeYes {
		if !stdinIsTerminal() {
			return fmt.Errorf("portage config differs from flavor %q and no terminal is available, run 'epull sync -y'", check.Flavor)
		}
		fmt.Printf("flavor %q on the server is out of sync with your portage config, sync it now? [y/N] ", check.Flavor)
		answer := ""
		if _, err := fmt.Scanln(&answer); err != nil {
			return fmt.Errorf("invalid input: %w, run 'epull sync -y' to sync without prompting", err)
		}
		if answer != "y" && answer != "yes" {
			log.Println("skipping portage sync")
			return nil
		}
	}

	if err := UploadPortage(fingerprint, archive); err != nil {
		return err
	}
	log.Println("portage config synced")
	return nil
}

func HandleSync(assumeYes, insecure bool) (error, int) {
	return cli.MustRegister([]cli.InitStep{
		{Name: "mtls client", Function: func() error { return InitializeMtlsClient(insecure) }},
		{Name: "portage sync", Function: func() error { return runSync(assumeYes) }},
	})
}

func writePortagePath(writer *tar.Writer, manifest *protocol.PortageManifest, relativePath string) error {
	path := filepath.Join(storage.PortageConfigRoot, relativePath)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return writePortageEntry(writer, manifest, path, relativePath, info)
}

// portage loves symlinks (profile -> /usr/portage/profile/...), follow them into the archive
func writePortageEntry(writer *tar.Writer, manifest *protocol.PortageManifest, path, relativePath string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("dangling symlink in portage config: %s", relativePath)
		}
		info = resolved
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		return fmt.Errorf("unsupported portage config entry: %s", relativePath)
	}

	var data []byte
	if info.IsDir() {
		manifest.AddDirectory(relativePath)
	} else {
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		manifest.AddFile(relativePath, int64(len(data)), sum[:])
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(relativePath)
	header.ModTime = header.ModTime.UTC().Truncate(0)
	header.AccessTime = header.ModTime
	header.ChangeTime = header.ModTime
	header.Uid = 0
	header.Gid = 0
	header.Uname = ""
	header.Gname = ""
	if info.IsDir() {
		header.Mode = 0755
	} else {
		header.Mode = 0644
	}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			childRelative := filepath.Join(relativePath, entry.Name())
			childPath := filepath.Join(storage.PortageConfigRoot, childRelative)
			childInfo, err := os.Lstat(childPath)
			if err != nil {
				return err
			}
			if err := writePortageEntry(writer, manifest, childPath, childRelative, childInfo); err != nil {
				return err
			}
		}
		return nil
	}

	_, err = writer.Write(data)
	return err
}
