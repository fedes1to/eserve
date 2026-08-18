package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/chroot"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/jobs"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

func PostProvision(w http.ResponseWriter, r *http.Request) {
	var provisionRequest protocol.ProvisionRequest
	if err := json.NewDecoder(r.Body).Decode(&provisionRequest); err != nil {
		http.Error(w, "couldn't decode provisionRequest", http.StatusBadRequest)
		return
	}

	identity := r.Context().Value(CtxKeyIdentity).(ClientIdentity)

	if !storage.MachineExists(identity.CN) {
		http.Error(w, "machine not registered, run identity first", http.StatusBadRequest)
		return
	}
	if chroot.IsGccMachineDiff(provisionRequest.GccMachine) {
		http.Error(w, "crossdev not supported, choose same arch as eserved", http.StatusBadRequest)
		return
	}

	job, err := jobs.Registry.Start(identity.CN, func(ctx context.Context, job *jobs.Job) {
		provisionJob(ctx, job, provisionRequest)
	})
	if err != nil {
		log.Printf("%v: racc failed to start provision job: %v\n", ClientIP(r), err)
		http.Error(w, "racc couldn't start provision job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(protocol.ProvisionResponse{
		JobID:      job.ID,
		BinhostURL: serverConfig.Settings.BaseBinhostURL,
	}); err != nil {
		log.Printf("%v: racc couldn't encode job response: %v\n", ClientIP(r), err)
	}
}

func provisionJob(ctx context.Context, job *jobs.Job, request protocol.ProvisionRequest) {
	job.WriteProgress("starting provision")

	if err := chroot.Provision(ctx, job, request); err != nil {
		job.Finish(jobs.StateError, protocol.StreamEvent{Type: "error", Message: err.Error()})
		return
	}

	if err := storage.ProvisionMachine(
		job.CN, request.Subarch, request.GccMachine, request.Profile, request.Flavor); err != nil {
		job.Finish(jobs.StateError, protocol.StreamEvent{Type: "error", Message: err.Error()})
		return
	}

	job.Finish(jobs.StateDone, protocol.StreamEvent{Type: "done", Message: "provision complete"})
}

func GetJobStream(w http.ResponseWriter, r *http.Request) {
	var jobRequest protocol.JobRequest
	if err := json.NewDecoder(r.Body).Decode(&jobRequest); err != nil {
		http.Error(w, "racc couldn't decode jobRequest", http.StatusBadRequest)
		return
	}

	job, ok := jobs.Registry.Get(jobRequest.JobID)
	if !ok {
		http.Error(w, "racc job not found", http.StatusNotFound)
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
		case <-job.Done():
			return
		case <-sub.Wait():
			for _, event := range sub.Pop() {
				writeEvent(w, flusher, event)
			}
		}
	}
}

func writeEvent(w http.ResponseWriter, flusher http.Flusher, event protocol.StreamEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}

	fmt.Fprintf(w, "event: %s\n", event.Type)
	fmt.Fprintf(w, "data: %s\n\n", payload)
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
		http.Error(w, "racc job not found", http.StatusNotFound)
		return
	}

	identity := r.Context().Value(CtxKeyIdentity).(ClientIdentity)
	if job.CN != identity.CN {
		http.Error(w, "racc not your job", http.StatusForbidden)
		return
	}

	if job.IsFinished() {
		http.Error(w, "racc job already finished", http.StatusConflict)
		return
	}

	job.Cancel()
	job.Finish(jobs.StateCancelled, protocol.StreamEvent{Type: "cancelled", Message: "cancelled by client"})

	w.WriteHeader(http.StatusOK)
}
