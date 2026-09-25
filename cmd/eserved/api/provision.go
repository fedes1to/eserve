package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/chroot"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/jobs"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/storage"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

func PostProvision(w http.ResponseWriter, r *http.Request) {
	var provisionRequest protocol.ProvisionRequest
	if !decodeJSONBody(w, r, &provisionRequest, "provisionRequest") {
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
	if !chroot.ValidFlavor(provisionRequest.Flavor) {
		http.Error(w, "invalid flavor", http.StatusBadRequest)
		return
	}
	// a broken cross.conf would otherwise fall through to the native arch gate
	if _, err := chroot.CrossTargetError(provisionRequest.Flavor); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if provisionRequest.GccMachine == "" {
		http.Error(w, "gcc_machine is required", http.StatusBadRequest)
		return
	}
	if chroot.IsGccMachineDiff(provisionRequest.GccMachine) && !chroot.CrossCoversArch(provisionRequest.Flavor, provisionRequest.GccMachine) {
		http.Error(w, "cross arch not supported for this flavor, choose same arch as eserved", http.StatusBadRequest)
		return
	}

	// the flavor the machine has right now; the job must still find it there
	expectedFlavor := machineFlavor
	var switchToken string
	var spentAt time.Time
	if machineFlavor != provisionRequest.Flavor {
		switchToken = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		// spent here, before the job touches anything, so a token can't be
		// reused by a concurrent request and a refused switch has no side effect
		var err error
		spentAt, err = storage.SpendFlavorSwitchToken(switchToken, identity.CN, provisionRequest.Flavor)
		if err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, storage.ErrTokenCN) || errors.Is(err, storage.ErrTokenFlavor) {
				status = http.StatusBadRequest
			}
			http.Error(w, err.Error(), status)
			return
		}
	}

	job, err := jobs.Registry.Start(identity.CN, provisionRequest.Flavor, "provision", func(ctx context.Context, job *jobs.Job) {
		ProvisionJob(ctx, job, provisionRequest, expectedFlavor)
	})
	if err != nil {
		// the job never started, so the switch never happened: hand the token back
		if switchToken != "" {
			if refundErr := storage.RefundFlavorSwitchToken(switchToken, identity.CN, spentAt); refundErr != nil {
				log.Printf("%v: racc couldn't refund the switch token: %v\n", ClientIP(r), refundErr)
			}
		}
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

func ProvisionJob(ctx context.Context, job *jobs.Job, request protocol.ProvisionRequest, authorizedFlavor string) {
	job.WriteProgress("starting provision")

	if err := chroot.Provision(ctx, job, request); err != nil {
		job.Finish(jobs.StateError, protocol.StreamEvent{Type: "error", Message: err.Error()})
		return
	}

	if err := storage.ProvisionMachine(
		job.CN, request.Subarch, request.GccMachine, request.Profile, request.Flavor, authorizedFlavor); err != nil {
		job.Finish(jobs.StateError, protocol.StreamEvent{Type: "error", Message: err.Error()})
		return
	}

	job.Finish(jobs.StateDone, protocol.StreamEvent{Type: "done", Message: "provision complete"})
}
