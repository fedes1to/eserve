package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

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
	machineFlavor, _ := storage.MachineFlavor(identity.CN)
	if provisionRequest.Flavor == "" {
		provisionRequest.Flavor = machineFlavor
	}

	token := ""
	if machineFlavor != provisionRequest.Flavor {
		token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !storage.IsTokenAvailable(token) {
			http.Error(w, "new token required for flavor", http.StatusUnauthorized)
			return
		}
	}
	if chroot.IsGccMachineDiff(provisionRequest.GccMachine) {
		http.Error(w, "crossdev not supported, choose same arch as eserved", http.StatusBadRequest)
		return
	}

	job, err := jobs.Registry.Start(identity.CN, provisionRequest.Flavor, "provision", func(ctx context.Context, job *jobs.Job) {
		ProvisionJob(ctx, job, provisionRequest, token)
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
		BinhostURL: strings.TrimRight(serverConfig.Settings.BaseBinhostURL, "/") + "/" + provisionRequest.Flavor,
		Flavor:     provisionRequest.Flavor,
	}); err != nil {
		log.Printf("%v: racc couldn't encode job response: %v\n", ClientIP(r), err)
	}
}

func ProvisionJob(ctx context.Context, job *jobs.Job, request protocol.ProvisionRequest, token string) {
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

	if token != "" {
		if err := storage.UseToken(token, job.CN); err != nil {
			job.Finish(jobs.StateError, protocol.StreamEvent{Type: "error", Message: err.Error()})
			return
		}
	}

	job.Finish(jobs.StateDone, protocol.StreamEvent{Type: "done", Message: "provision complete"})
}
