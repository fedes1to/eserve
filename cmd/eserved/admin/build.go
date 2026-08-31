package admin

import (
	"context"
	"encoding/json"
	"net/http"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/chroot"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/jobs"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

func PostStartBuild(w http.ResponseWriter, r *http.Request) {
	var buildRequest protocol.BuildRequest
	if err := json.NewDecoder(r.Body).Decode(&buildRequest); err != nil {
		http.Error(w, "couldn't decode buildRequest", http.StatusBadRequest)
		return
	}

	if len(buildRequest.Packages) == 0 {
		http.Error(w, "no packages to build", http.StatusBadRequest)
		return
	}
	if !chroot.IsProvisioned(buildRequest.Flavor) {
		http.Error(w, "flavor not provisioned", http.StatusNotFound)
		return
	}

	job, err := jobs.Registry.Start("", buildRequest.Flavor, "build", func(ctx context.Context, job *jobs.Job) {
		if err := chroot.BuildJob(ctx, job, buildRequest.Flavor, buildRequest.Packages); err != nil {
			job.Finish(jobs.StateError, protocol.StreamEvent{Type: "error", Message: err.Error()})
		}
	})
	if err != nil {
		http.Error(w, "failed to start build, check logs", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(protocol.BuildResponse{JobID: job.ID})
}
