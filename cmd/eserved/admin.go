package main

// used for eservectl comms
import (
	"fmt"
	"net/http"

	"git.fedesito.me/fedes1to/eserve/internal/config"
)

func postCreateToken(w http.ResponseWriter, r *http.Request) {
	token, tokenError := config.CreateToken()

	if tokenError != nil {
		http.Error(w, "failed to create token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, token)
}
