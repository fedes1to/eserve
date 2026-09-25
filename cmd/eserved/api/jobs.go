package api

import (
	"net/http"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/jobs"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

func GetJobStream(w http.ResponseWriter, r *http.Request) {
	var jobRequest protocol.JobRequest
	if !decodeJSONBody(w, r, &jobRequest, "jobRequest") {
		return
	}

	job, ok := jobs.Registry.Get(jobRequest.JobID)
	if !ok {
		http.Error(w, "racc's job not found", http.StatusNotFound)
		return
	}

	identity := r.Context().Value(CtxKeyIdentity).(ClientIdentity)
	if job.CN != identity.CN {
		http.Error(w, "racc not your job", http.StatusForbidden)
		return
	}

	// a client that goes away mid-stream is nothing to do about
	_ = job.Stream(w, r.Context().Done())
}

func PostCancelJob(w http.ResponseWriter, r *http.Request) {
	var jobRequest protocol.JobRequest
	if !decodeJSONBody(w, r, &jobRequest, "jobRequest") {
		return
	}

	job, ok := jobs.Registry.Get(jobRequest.JobID)
	if !ok {
		http.Error(w, "racc's job not found", http.StatusNotFound)
		return
	}

	identity := r.Context().Value(CtxKeyIdentity).(ClientIdentity)
	if job.CN != identity.CN {
		http.Error(w, "racc not your job", http.StatusForbidden)
		return
	}

	if job.IsFinished() {
		http.Error(w, "racc's job already finished", http.StatusConflict)
		return
	}

	job.Cancel()
	job.Finish(jobs.StateCancelled, protocol.StreamEvent{Type: "cancelled", Message: "cancelled by client"})

	w.WriteHeader(http.StatusOK)
}
