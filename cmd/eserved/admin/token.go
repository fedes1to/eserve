package admin

// used for eservectl comms
import (
	"fmt"
	"net/http"
	"os"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
)

func PostCreateToken(w http.ResponseWriter, r *http.Request) {
	token, err := storage.CreateToken()

	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to create token,", err)
		http.Error(w, "failed to create token, check logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, token)
}
