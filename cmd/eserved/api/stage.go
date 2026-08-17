package api

import (
	"fmt"
	"net/http"
	"os"

	"git.fedesito.me/fedes1to/eserve/internal/sharedStorage"
)

func GetStages(w http.ResponseWriter, r *http.Request) {
	stageList, err := sharedStorage.GetStageList()

	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to get stages,", err)
		http.Error(w, "failed to get stages, check logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	for _, stage := range stageList {
		fmt.Fprintln(w, stage)
	}
}
