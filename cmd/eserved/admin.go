package main

// used for eservectl comms
import (
	"fmt"
	"net/http"
	"os"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
)

func postCreateToken(w http.ResponseWriter, r *http.Request) {
	token, tokenError := storage.CreateToken()

	if tokenError != nil {
		fmt.Fprintln(os.Stderr, "failed to create token,", tokenError)
		http.Error(w, "failed to create token, check logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, token)
}
