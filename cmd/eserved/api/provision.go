package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	if refusal := chroot.ArchRefusal(provisionRequest.Flavor, provisionRequest.GccMachine); refusal != "" {
		http.Error(w, refusal, http.StatusBadRequest)
		return
	}

	// the flavor the machine has right now; the job must still find it there
	expectedFlavor := machineFlavor
	var switchToken *storage.SwitchToken
	if machineFlavor != provisionRequest.Flavor {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		// spent here, before the job touches anything, so a token can't be
		// reused by a concurrent request and a refused switch has no side effect
		var err error
		switchToken, err = storage.SpendFlavorSwitchToken(token, identity.CN, provisionRequest.Flavor)
		if err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, storage.ErrTokenCN) || errors.Is(err, storage.ErrTokenFlavor) ||
				errors.Is(err, storage.ErrFlavorExists) {
				status = http.StatusBadRequest
			}
			http.Error(w, err.Error(), status)
			return
		}
	}

	job, err := jobs.Registry.Start(identity.CN, provisionRequest.Flavor, "provision", func(ctx context.Context, job *jobs.Job) {
		ProvisionJob(ctx, job, provisionRequest, expectedFlavor, switchToken)
	})
	if err != nil {
		// the job never started, so the switch never happened: hand the token back
		if refundErr := switchToken.Refund(); refundErr != nil {
			log.Printf("%v: racc couldn't refund the switch token: %v\n", ClientIP(r), refundErr)
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

func ProvisionJob(ctx context.Context, job *jobs.Job, request protocol.ProvisionRequest, authorizedFlavor string, switchToken *storage.SwitchToken) {
	job.WriteProgress("starting provision")

	// the switch only counts once ProvisionMachine wrote it, so anything before
	// that hands the token back; the defer catches a panic too
	defer func() { _ = switchToken.Refund() }()

	err := provisionChrootAndMachine(ctx, job, request, authorizedFlavor)
	if err == nil {
		switchToken.Commit()
	}
	if refundErr := switchToken.Refund(); refundErr != nil {
		job.WriteProgress("warning: couldn't refund the switch token: " + refundErr.Error())
	}
	if err != nil {
		job.Finish(jobs.StateError, protocol.StreamEvent{Type: "error", Message: err.Error()})
		return
	}

	// machines with different profiles make each other's binpkgs useless
	if profiles := storage.FlavorProfiles(request.Flavor); len(profiles) > 1 {
		job.WriteProgress(fmt.Sprintf("warning: flavor %s has machines with different profiles (%s), binpkgs built with one are ignored by the others",
			request.Flavor, strings.Join(profiles, ", ")))
	}

	job.Finish(jobs.StateDone, protocol.StreamEvent{Type: "done", Message: "provision complete"})
}

// the chroot work and the machine record; the flavor switch is only written by the last step
func provisionChrootAndMachine(ctx context.Context, job *jobs.Job, request protocol.ProvisionRequest, authorizedFlavor string) error {
	if err := chroot.Provision(ctx, job, request); err != nil {
		return err
	}
	return storage.ProvisionMachine(
		job.CN, request.Subarch, request.GccMachine, request.Profile, request.Flavor, authorizedFlavor)
}
