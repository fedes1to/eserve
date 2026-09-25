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
	if err := json.NewDecoder(r.Body).Decode(&revokeRequest); err != nil {
		http.Error(w, "couldn't decode revokeRequest", http.StatusBadRequest)
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
	if err := json.NewDecoder(r.Body).Decode(&deleteRequest); err != nil {
		http.Error(w, "couldn't decode deleteRequest", http.StatusBadRequest)
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
