package api

import (
	"net/http"

	"git.fedesito.me/fedes1to/eserve/internal/gpg"
)

func GetSigningKey(w http.ResponseWriter, r *http.Request) {
	key, err := gpg.PublicKey()
	if err != nil {
		http.Error(w, "no signing key available", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(key))
}
