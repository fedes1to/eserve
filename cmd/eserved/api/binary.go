package api

import (
	"encoding/json"
	"net/http"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
)

// GET /api/v1/binary?name=epull&arch=x86_64-pc-linux-gnu
func GetBinary(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	arch := r.URL.Query().Get("arch")
	if name == "" || arch == "" {
		http.Error(w, "name and arch are required", http.StatusBadRequest)
		return
	}

	path, _, err := storage.GetBinary(name, arch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, path)
}

// GET /api/v1/binary/manifest?name=epull&arch=x86_64-pc-linux-gnu
func GetBinaryManifest(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	arch := r.URL.Query().Get("arch")
	if name == "" || arch == "" {
		http.Error(w, "name and arch are required", http.StatusBadRequest)
		return
	}

	_, manifest, err := storage.GetBinary(name, arch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(manifest)
}
