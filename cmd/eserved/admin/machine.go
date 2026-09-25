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

func PostRevokeMachine(w http.ResponseWriter, r *http.Request) {
	var revokeRequest protocol.RevokeRequest
	if !decodeJSONBody(w, r, &revokeRequest, "revokeRequest") {
		return
	}
	err := storage.RevokeMachine(revokeRequest.CN)

	if err != nil {
		log.Println("failed to revoke machine,", err)
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "failed to revoke machine, check logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "ok")
}

func PostListMachines(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(protocol.MachineListResponse{Machines: storage.ListMachines()})
}

func PostDeleteMachine(w http.ResponseWriter, r *http.Request) {
	var deleteRequest protocol.DeleteMachineRequest
	if !decodeJSONBody(w, r, &deleteRequest, "deleteRequest") {
		return
	}
	err := storage.DeleteMachine(deleteRequest.CN)

	if err != nil {
		log.Println("failed to delete machine,", err)
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete machine, check logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "ok")
}
