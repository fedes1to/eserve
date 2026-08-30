package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

func PostUploadBinary(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, protocol.MaxBinarySize)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "couldn't parse the upload: "+err.Error(), http.StatusBadRequest)
		return
	}

	var manifest protocol.BinaryManifest
	if err := json.Unmarshal([]byte(r.FormValue("manifest")), &manifest); err != nil {
		http.Error(w, "couldn't decode the manifest: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("binary")
	if err != nil {
		http.Error(w, "no binary file in the upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// the client doesn't decide when it uploaded, we do
	manifest.UploadedAt = time.Now().Format(time.RFC3339)

	if err := storage.UploadBinary(manifest, file); err != nil {
		http.Error(w, "failed to upload binary: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func PostListBinaries(w http.ResponseWriter, r *http.Request) {
	binaries, err := storage.ListBinaries()
	if err != nil {
		http.Error(w, "failed to list binaries: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(protocol.BinaryListResponse{Binaries: binaries})
}
