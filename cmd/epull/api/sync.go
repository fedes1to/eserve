package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"git.fedesito.me/fedes1to/eserve/cmd/epull/clientConfig"
	"git.fedesito.me/fedes1to/eserve/internal/urls"
)

const portageConfigRoot = "/etc/portage"

var portageSyncPaths = []string{
	"make.conf",
	"package.use",
	"package.accept_keywords",
	"package.mask",
	"package.unmask",
	"package.license",
	"package.properties",
	"package.env",
	"env",
	"sets",
	"savedconfig",
	"patches",
	"repos.conf",
	"binrepos.conf",
}

func SyncPortage() (fingerprint string, archive io.Reader, err error) {
	var archiveData bytes.Buffer
	var manifest bytes.Buffer
	gzipWriter := gzip.NewWriter(&archiveData)
	tarWriter := tar.NewWriter(gzipWriter)

	paths := append([]string(nil), portageSyncPaths...)
	sort.Strings(paths)
	for _, relativePath := range paths {
		if err := writePortagePath(tarWriter, &manifest, relativePath); err != nil {
			tarWriter.Close()
			gzipWriter.Close()
			return "", nil, err
		}
	}
	if err := tarWriter.Close(); err != nil {
		return "", nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return "", nil, err
	}

	digest := sha256.Sum256(manifest.Bytes())
	return hex.EncodeToString(digest[:]), bytes.NewReader(archiveData.Bytes()), nil
}

func CheckSync() (bool, error) {
	fingerprint, _, err := SyncPortage()
	if err != nil {
		return false, err
	}

	request, err := http.NewRequest("POST", clientConfig.Settings.Server+urls.CheckSyncSuburl, nil)
	if err != nil {
		return false, err
	}
	request.Header.Set("X-Portage-Fingerprint", fingerprint)
	response, err := mtlsClient.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent {
		return true, nil
	}
	if response.StatusCode == http.StatusConflict {
		return false, nil
	}
	body, _ := io.ReadAll(response.Body)
	return false, fmt.Errorf("couldn't check portage config, code %v, body: %s", response.StatusCode, string(body))
}

func UploadPortage() error {
	_, archive, err := SyncPortage()
	if err != nil {
		return err
	}

	request, err := http.NewRequest("POST", clientConfig.Settings.Server+urls.SyncSuburl, archive)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/gzip")

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

func writePortagePath(writer *tar.Writer, manifest io.Writer, relativePath string) error {
	path := filepath.Join(portageConfigRoot, relativePath)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return writePortageEntry(writer, manifest, path, relativePath, info)
}

func writePortageEntry(writer *tar.Writer, manifest io.Writer, path, relativePath string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() && !info.IsDir() {
		return fmt.Errorf("unsupported portage config entry: %s", relativePath)
	}

	if info.IsDir() {
		if _, err := fmt.Fprintf(manifest, "%s\tdirectory\n", filepath.ToSlash(relativePath)); err != nil {
			return err
		}
	} else {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		fileDigest := sha256.New()
		if _, err := io.Copy(fileDigest, file); err != nil {
			file.Close()
			return err
		}
		file.Close()
		if _, err := fmt.Fprintf(manifest, "%s\tfile\t%d\t%s\n", filepath.ToSlash(relativePath), info.Size(), hex.EncodeToString(fileDigest.Sum(nil))); err != nil {
			return err
		}
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
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			childRelative := filepath.Join(relativePath, entry.Name())
			childPath := filepath.Join(portageConfigRoot, childRelative)
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

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}
