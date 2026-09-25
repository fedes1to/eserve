package admin

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/chroot"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
)

type flavorApplyRequest struct {
	Flavor string `json:"flavor"`
}

func PostDeleteFlavor(w http.ResponseWriter, r *http.Request) {
	var request flavorApplyRequest
	if !decodeJSONBody(w, r, &request, "flavorDeleteRequest") {
		return
	}
	if !chroot.ValidFlavor(request.Flavor) {
		http.Error(w, "invalid flavor", http.StatusBadRequest)
		return
	}
	// a machine on the flavor would be left pointing at a chroot that no longer exists
	if machines := storage.MachinesOnFlavor(request.Flavor); len(machines) > 0 {
		http.Error(w, fmt.Sprintf("flavor %s still has machines on it (%s), delete or move them first",
			request.Flavor, strings.Join(machines, ", ")), http.StatusConflict)
		return
	}
	if err := chroot.DeleteFlavor(request.Flavor); err != nil {
		http.Error(w, "failed to delete flavor: "+err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("ok"))
}

func PostApplyFlavor(w http.ResponseWriter, r *http.Request) {
	var request flavorApplyRequest
	if !decodeJSONBody(w, r, &request, "flavorApplyRequest") {
		return
	}

	if !chroot.IsProvisioned(request.Flavor) {
		http.Error(w, "flavor not provisioned", http.StatusNotFound)
		return
	}

	archives := chroot.ClientSyncArchives(request.Flavor)

	// last synced client goes last, its config ends up on top
	profile := ""
	if fingerprint, ok := storage.FlavorFingerprintInfo(request.Flavor); ok {
		archive := chroot.SyncArchivePath(request.Flavor, fingerprint.SyncedBy)
		var rest []string
		for _, a := range archives {
			if a != archive {
				rest = append(rest, a)
			}
		}
		archives = append(rest, archive)
		profile, _ = storage.MachineProfile(fingerprint.SyncedBy)
	}

	if err := chroot.ApplyFlavorToChroot(context.Background(), request.Flavor, archives, profile); err != nil {
		w.Header().Set("Content-Type", "text/plain")
		http.Error(w, "failed to apply flavor config, check logs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("ok"))
}
