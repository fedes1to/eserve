package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

func PostCreateToken(w http.ResponseWriter, r *http.Request) {
	token, err := storage.CreateToken()

	if err != nil {
		log.Println("failed to create token,", err)
		http.Error(w, "failed to create token, check logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, token)
}

func PostListTokens(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(protocol.TokenListResponse{Tokens: storage.ListTokens()})
}

func PostDeleteToken(w http.ResponseWriter, r *http.Request) {
	var deleteRequest protocol.DeleteTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&deleteRequest); err != nil {
		http.Error(w, "couldn't decode deleteRequest", http.StatusBadRequest)
		return
	}
	err := storage.DeleteToken(deleteRequest.Token)

	if err != nil {
		log.Println("failed to delete token,", err)
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete token, check logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "ok")
}
