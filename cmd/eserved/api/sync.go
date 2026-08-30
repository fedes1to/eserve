package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/chroot"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

const maxSyncArchiveSize = 64 << 20

func PostCheckSync(w http.ResponseWriter, r *http.Request) {
	identity := r.Context().Value(CtxKeyIdentity).(ClientIdentity)
	flavor, ok := storage.MachineFlavor(identity.CN)
	if !ok || flavor == "" {
		http.Error(w, "machine has no flavor, run provision first", http.StatusBadRequest)
		return
	}

	fingerprint := r.Header.Get("X-Portage-Fingerprint")
	if fingerprint == "" {
		http.Error(w, "portage fingerprint required", http.StatusBadRequest)
		return
	}

	stored, hasStored := storage.GetFlavorFingerprint(flavor)
	if hasStored && stored == fingerprint {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	if err := json.NewEncoder(w).Encode(protocol.SyncCheckResponse{
		Flavor:      flavor,
		Fingerprint: stored,
	}); err != nil {
		log.Printf("%v: racc couldn't encode check response: %v\n", ClientIP(r), err)
	}
}

func PostSync(w http.ResponseWriter, r *http.Request) {
	identity := r.Context().Value(CtxKeyIdentity).(ClientIdentity)
	flavor, ok := storage.MachineFlavor(identity.CN)
	if !ok || flavor == "" {
		http.Error(w, "machine has no flavor, run provision first", http.StatusBadRequest)
		return
	}

	claimed := r.Header.Get("X-Portage-Fingerprint")
	if claimed == "" {
		http.Error(w, "portage fingerprint required", http.StatusBadRequest)
		return
	}

	if !chroot.IsProvisioned(flavor) {
		http.Error(w, "flavor is not provisioned, run provision first", http.StatusConflict)
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

	fingerprint, err := chroot.ApplySync(r.Context(), flavor, claimed, archivePath)
	if err != nil {
		log.Printf("%v: racc failed to apply sync for flavor %v: %v\n", ClientIP(r), flavor, err)
		http.Error(w, "racc couldn't apply sync to chroot", http.StatusInternalServerError)
		return
	}

	if err := storage.SetFlavorFingerprint(flavor, fingerprint, identity.CN); err != nil {
		log.Printf("%v: racc failed to record fingerprint for flavor %v: %v\n", ClientIP(r), flavor, err)
		http.Error(w, "couldn't record portage fingerprint", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
