package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
)

const maxSyncArchiveSize = 64 << 20

func PostCheckSync(w http.ResponseWriter, r *http.Request) {
	fingerprint := r.Header.Get("X-Portage-Fingerprint")
	if fingerprint == "" {
		http.Error(w, "portage fingerprint required", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func PostSync(w http.ResponseWriter, r *http.Request) {
	identity := r.Context().Value(CtxKeyIdentity).(ClientIdentity)
	flavor, ok := storage.MachineFlavor(identity.CN)
	if !ok || flavor == "" {
		http.Error(w, "machine has no flavor", http.StatusBadRequest)
		return
	}
	syncDir := filepath.Join(serverConfig.ServerConfigPath, "sync", flavor)
	if err := os.MkdirAll(syncDir, 0700); err != nil {
		http.Error(w, "couldn't create sync directory", http.StatusInternalServerError)
		return
	}

	archivePath := filepath.Join(syncDir, identity.CN+".tar.gz")
	temporary, err := os.CreateTemp(syncDir, ".sync-*.tar.gz")
	if err != nil {
		http.Error(w, "couldn't create sync archive", http.StatusInternalServerError)
		return
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	defer temporary.Close()

	limited := io.LimitReader(r.Body, maxSyncArchiveSize+1)
	written, err := io.Copy(temporary, limited)
	if err != nil {
		http.Error(w, "couldn't receive sync archive", http.StatusBadRequest)
		return
	}
	if written > maxSyncArchiveSize {
		http.Error(w, fmt.Sprintf("sync archive exceeds %d bytes", maxSyncArchiveSize), http.StatusRequestEntityTooLarge)
		return
	}
	if err := temporary.Close(); err != nil {
		http.Error(w, "couldn't close sync archive", http.StatusInternalServerError)
		return
	}
	if err := os.Rename(temporaryPath, archivePath); err != nil {
		http.Error(w, "couldn't store sync archive", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
