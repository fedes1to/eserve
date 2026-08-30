package chroot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/jobs"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/flavorlock"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

// proves the chroot got a real stage3, a synced config can't fake it
const stageMarker = "eserved-stage3.json"

type stageMarkerData struct {
	Stagefile   string    `json:"stagefile"`
	ExtractedAt time.Time `json:"extracted_at"`
}

func writeStageMarker(chrootDir, stagefile string) error {
	root, err := os.OpenRoot(chrootDir)
	if err != nil {
		return err
	}
	defer root.Close()

	marker, err := json.Marshal(stageMarkerData{Stagefile: stagefile, ExtractedAt: time.Now()})
	if err != nil {
		return err
	}
	return root.WriteFile("etc/"+stageMarker, marker, 0o644)
}

func Provision(ctx context.Context, job *jobs.Job, request protocol.ProvisionRequest) error {
	if filepath.Base(request.Stagefile) != request.Stagefile {
		return fmt.Errorf("invalid stagefile %q: must be a plain filename", request.Stagefile)
	}

	stagePath := filepath.Join(serverConfig.Settings.StagePath, request.Stagefile)
	if _, err := os.Stat(stagePath); err != nil {
		return fmt.Errorf("stagefile not found: %w", err)
	}

	if IsProvisioned(request.Flavor) {
		job.WriteProgress("chroot already exists, skipping extraction")
	} else {
		dir := chrootDir(request.Flavor)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("couldn't create chroot dir: %w", err)
		}

		job.WriteProgress(fmt.Sprintf("extracting stage3 from %s", request.Stagefile))

		if err := extractStage3(ctx, job, stagePath, dir); err != nil {
			return fmt.Errorf("couldn't extract stage3: %w", err)
		}

		if err := writeStageMarker(dir, request.Stagefile); err != nil {
			return fmt.Errorf("couldn't mark chroot as provisioned: %w", err)
		}
	}

	return restoreSyncedConfig(ctx, job, request.Flavor, job.CN)
}

func restoreSyncedConfig(ctx context.Context, job *jobs.Job, flavor, cn string) error {
	if !validFlavor(flavor) {
		return fmt.Errorf("invalid flavor %q", flavor)
	}
	unlock := flavorlock.Lock(flavor)
	defer unlock()

	root, err := os.OpenRoot(chrootDir(flavor))
	if err != nil {
		return fmt.Errorf("couldn't open chroot: %w", err)
	}
	defer root.Close()

	sweepStaleConfigDirs(root, chrootDir(flavor))

	if _, err := root.Lstat(ConfigDir); err == nil {
		job.WriteProgress("re-linking portage config")
		return linkSyncedPathsFromDir(root, chrootDir(flavor))
	}

	archivePath := SyncArchivePath(flavor, cn)
	hasArchive := fileExists(archivePath)
	if !hasArchive && !hasFlavorConfig(flavor) {
		return nil // nothing to install
	}

	if hasArchive {
		job.WriteProgress("restoring portage config from stored archive")
	} else {
		job.WriteProgress("installing flavor config")
	}
	suffix, err := randomSuffix()
	if err != nil {
		return err
	}
	staging := ConfigDir + ".new-" + suffix

	archive := ""
	if hasArchive {
		archive = archivePath
	}
	present, _, err := buildStagedConfig(ctx, root, staging, flavor, archive)
	if err != nil {
		root.RemoveAll(staging)
		return err
	}
	return installStagedConfig(root, staging, present)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func linkSyncedPathsFromDir(root *os.Root, chrootDir string) error {
	entries, err := os.ReadDir(filepath.Join(chrootDir, ConfigDir))
	if err != nil {
		return fmt.Errorf("couldn't read config dir: %w", err)
	}
	present := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if syncedPathSet[entry.Name()] {
			present[entry.Name()] = true
		}
	}
	return linkSyncedPaths(root, present)
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
