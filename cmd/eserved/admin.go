package main

// used for eservectl comms
import (
	"fmt"
	"net/http"
	"os"

	serverConfig "git.fedesito.me/fedes1to/eserve/cmd/eserved/config"
)

func postCreateToken(w http.ResponseWriter, r *http.Request) {
	token, tokenError := serverConfig.CreateToken()

	if tokenError != nil {
		fmt.Fprintln(os.Stderr, "failed to create token,", tokenError)
		http.Error(w, "failed to create token, check logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, token)
}
