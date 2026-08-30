package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"git.fedesito.me/fedes1to/eserve/internal/protocol"
	"git.fedesito.me/fedes1to/eserve/internal/sysinfo"
	"git.fedesito.me/fedes1to/eserve/internal/urls"
)

// replaces the running epull binary with the build the server hosts for this machine's arch
func HandleSelfUpdate() (error, int) {
	arch, err := sysinfo.GetGccMachine()
	if err != nil {
		return err, 1
	}

	// 404 means the arch was never uploaded, anything else is a real problem
	var manifest protocol.BinaryManifest
	status, err := sendMtlsGet(urls.BinaryManifestSuburl+"?name=epull&arch="+url.QueryEscape(arch), &manifest)
	if err != nil {
		return err, 1
	}
	if status == http.StatusNotFound {
		return fmt.Errorf("server has no %s build of epull, upload one with 'eservectl binary upload'", arch), 1
	}

	executable, err := os.Executable()
	if err != nil {
		return err, 1
	}

	// temp file lands next to the running binary so the rename stays on one filesystem
	downloadURL := urls.BinarySuburl + "?name=epull&arch=" + url.QueryEscape(arch)
	response, err := sendMtlsGetRaw(downloadURL)
	if err != nil {
		return err, 1
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("binary download failed with status %d", response.StatusCode), 1
	}

	tempFile, err := os.CreateTemp(filepath.Dir(executable), "epull-update-*")
	if err != nil {
		return err, 1
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath) // no-op once the rename succeeded

	// manifest.Size + 1: reading that many means the server sent more than it advertised
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tempFile, hasher), io.LimitReader(response.Body, manifest.Size+1))
	if err != nil {
		tempFile.Close()
		return err, 1
	}
	tempFile.Close()
	if written > manifest.Size {
		return fmt.Errorf("server sent %d bytes, manifest advertises %d", written, manifest.Size), 1
	}

	actualSum := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actualSum, manifest.SHA256) {
		return fmt.Errorf("sha256 mismatch: got %s, manifest says %s", actualSum, manifest.SHA256), 1
	}

	if err := os.Chmod(tempPath, 0755); err != nil {
		return err, 1
	}
	// the running process keeps its own inode alive, overwriting the path is safe
	if err := os.Rename(tempPath, executable); err != nil {
		return err, 1
	}

	fmt.Printf("replaced %v with %s build (sha256 %s, %d bytes)\n", executable, arch, actualSum, manifest.Size)
	return nil, 0
}
