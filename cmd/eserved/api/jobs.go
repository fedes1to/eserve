package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/jobs"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

func GetJobStream(w http.ResponseWriter, r *http.Request) {
	var jobRequest protocol.JobRequest
	if err := json.NewDecoder(r.Body).Decode(&jobRequest); err != nil {
		http.Error(w, "racc couldn't decode jobRequest", http.StatusBadRequest)
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

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	sub, endOffset, err := job.Subscribe()
	if err != nil {
		return
	}
	defer job.Unsubscribe(sub)

	if err := job.Replay(func(event protocol.StreamEvent) error {
		writeEvent(w, flusher, event)
		return nil
	}, endOffset); err != nil {
		return
	}
	if job.IsFinished() {
		return
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-sub:
			if !ok {
				return // racc's job is over
			}
			writeEvent(w, flusher, event)
		}
	}
}

func writeEvent(w http.ResponseWriter, flusher http.Flusher, event protocol.StreamEvent) {
	fmt.Fprintln(w, protocol.Colorize(event))
	flusher.Flush()
}

func PostCancelJob(w http.ResponseWriter, r *http.Request) {
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
