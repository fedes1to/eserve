package api

import (
	"net/http"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
)

// the CA is public; epull pins it during registration and verifies against it after
func GetCa(w http.ResponseWriter, r *http.Request) {
	if len(storage.CaCertificatePEM) == 0 {
		http.Error(w, "no CA available", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write(storage.CaCertificatePEM)
}
