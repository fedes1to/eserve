package chroot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/jobs"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

func Provision(ctx context.Context, job *jobs.Job, request protocol.ProvisionRequest) error {
	if filepath.Base(request.Stagefile) != request.Stagefile {
		return fmt.Errorf("invalid stagefile %q: must be a plain filename", request.Stagefile)
	}

	chrootDir := filepath.Join(serverConfig.Settings.ChrootBase, request.Flavor)

	stagePath := filepath.Join(serverConfig.Settings.StagePath, request.Stagefile)
	if _, err := os.Stat(stagePath); err != nil {
		return fmt.Errorf("stagefile not found: %w", err)
	}

	if _, err := os.Stat(filepath.Join(chrootDir, "etc")); err == nil {
		job.WriteProgress("chroot already exists, skipping extraction")
		return nil
	}

	if err := os.MkdirAll(chrootDir, 0755); err != nil {
		return fmt.Errorf("couldn't create chroot dir: %w", err)
	}

	job.WriteProgress(fmt.Sprintf("extracting stage3 from %s", request.Stagefile))

	if err := extractStage3(ctx, job, stagePath, chrootDir); err != nil {
		return fmt.Errorf("couldn't extract stage3: %w", err)
	}

	job.WriteProgress("provisioning complete")
	return nil
}

func extractStage3(ctx context.Context, job *jobs.Job, stagePath, dest string) error {
	command := exec.CommandContext(ctx, "tar", "-xpf", stagePath, "-C", dest)
	command.Stdout = &jobs.JobWriter{Job: job}
	command.Stderr = &jobs.JobWriter{Job: job}

	start := time.Now()
	if err := command.Run(); err != nil {
		return err
	}
	job.WriteOutput(fmt.Sprintf("extraction took %s", time.Since(start)))
	return nil
}
