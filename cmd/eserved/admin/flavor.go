package admin

import (
	"context"
	"encoding/json"
	"net/http"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/chroot"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
)

type flavorApplyRequest struct {
	Flavor string `json:"flavor"`
}

func PostApplyFlavor(w http.ResponseWriter, r *http.Request) {
	var request flavorApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "couldn't decode flavorApplyRequest", http.StatusBadRequest)
		return
	}

	if !chroot.IsProvisioned(request.Flavor) {
		http.Error(w, "flavor not provisioned", http.StatusNotFound)
		return
	}

	archives := chroot.ClientSyncArchives(request.Flavor)

	// last synced client goes last, its config ends up on top
	if fingerprint, ok := storage.FlavorFingerprintInfo(request.Flavor); ok {
		archive := chroot.SyncArchivePath(request.Flavor, fingerprint.SyncedBy)
		var rest []string
		for _, a := range archives {
			if a != archive {
				rest = append(rest, a)
			}
		}
		archives = append(rest, archive)
	}

	if err := chroot.ApplyFlavorToChroot(context.Background(), request.Flavor, archives); err != nil {
		w.Header().Set("Content-Type", "text/plain")
		http.Error(w, "failed to apply flavor config, check logs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("ok"))
}
