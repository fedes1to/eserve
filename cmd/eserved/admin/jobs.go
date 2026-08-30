package admin

import (
	"encoding/json"
	"net/http"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/jobs"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

func PostListJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(protocol.JobListResponse{Jobs: jobs.Registry.List()})
}

func PostAdminCancelJob(w http.ResponseWriter, r *http.Request) {
	var jobRequest protocol.JobRequest
	if err := json.NewDecoder(r.Body).Decode(&jobRequest); err != nil {
		http.Error(w, "couldn't decode jobRequest", http.StatusBadRequest)
		return
	}

	job, ok := jobs.Registry.Get(jobRequest.JobID)
	if !ok {
		http.Error(w, "racc's job not found", http.StatusNotFound)
		return
	}

	if job.IsFinished() {
		http.Error(w, "racc's job already finished", http.StatusConflict)
		return
	}

	job.Cancel()
	job.Finish(jobs.StateCancelled, protocol.StreamEvent{Type: "cancelled", Message: "cancelled by admin"})

	w.WriteHeader(http.StatusOK)
}

func PostAdminJobStream(w http.ResponseWriter, r *http.Request) {
	var jobRequest protocol.JobRequest
	if err := json.NewDecoder(r.Body).Decode(&jobRequest); err != nil {
		http.Error(w, "couldn't decode jobRequest", http.StatusBadRequest)
		return
	}

	job, ok := jobs.Registry.Get(jobRequest.JobID)
	if !ok {
		http.Error(w, "racc's job not found", http.StatusNotFound)
		return
	}

	// no request context to watch here, a nil cancel channel never fires
	_ = job.Stream(w, nil)
}
