package chroot

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/jobs"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/flavorlock"
	"git.fedesito.me/fedes1to/eserve/internal/gpg"
)

// atoms end up on a chroot command line and in a PKGDIR path, keep them boring:
// no flags, no metacharacters, one slash, no traversal
var atomPattern = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*)/([A-Za-z0-9][A-Za-z0-9._-]*?)(-[0-9][A-Za-z0-9.+-]*)?(:[0-9][A-Za-z0-9.+-]*)?$`)

func validateBuildAtom(atom string) error {
	if len(atom) == 0 || len(atom) > 128 {
		return fmt.Errorf("invalid atom %q", atom)
	}
	if strings.ContainsAny(atom, ";&|$` \t\n") {
		return fmt.Errorf("invalid atom %q: no shell metacharacters", atom)
	}
	// portage's versioned atom syntax: =cat/pkg-version
	body := strings.TrimPrefix(atom, "=")
	if strings.Count(body, "/") != 1 || strings.Contains(body, "..") {
		return fmt.Errorf("invalid atom %q: must be cat/pkg", atom)
	}
	if !atomPattern.MatchString(body) {
		return fmt.Errorf("invalid atom %q: must be [=]cat/pkg[-version][:slot]", atom)
	}
	return nil
}

// the PKGDIR subdir an atom's gpkgs live in, refusing anything outside it
func atomPkgDir(flavor, atom string) (string, error) {
	if err := validateBuildAtom(atom); err != nil {
		return "", err
	}
	match := atomPattern.FindStringSubmatch(strings.TrimPrefix(atom, "="))
	base := binpkgDir(flavor)
	dir := filepath.Join(base, match[1], match[2])
	if !strings.HasPrefix(dir, base+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid atom %q: outside the binpkg dir", atom)
	}
	return dir, nil
}

// resolv/hosts binds are for distfile fetches
var chrootMounts = []struct{ host, chroot string }{
	{"/dev", "dev"},
	{"/proc", "proc"},
	{"/sys", "sys"},
	{"/etc/resolv.conf", "etc/resolv.conf"},
	{"/etc/hosts", "etc/hosts"},
}

// a crashed run can leave mounts behind, and mounting twice is an error
func isChrootMountActive(chrootDir, name string) bool {
	target := filepath.Join(chrootDir, name)
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// mountinfo: the mountpoint is field 5, before the " - "
		parts := strings.SplitN(scanner.Text(), " - ", 2)
		if len(parts) != 2 {
			continue
		}
		fields := strings.Fields(parts[0])
		if len(fields) >= 5 && fields[4] == target {
			return true
		}
	}
	return false
}

func mountChroot(flavor string) error {
	dir := chrootDir(flavor)
	for _, mount := range chrootMounts {
		target := filepath.Join(dir, mount.chroot)
		if isChrootMountActive(dir, mount.chroot) {
			continue // still mounted, whatever
		}
		if _, err := os.Stat(target); err != nil {
			// a bind needs its target to exist (resolv.conf)
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("couldn't create dir for %s: %w", mount.chroot, err)
			}
			file, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("couldn't create %s: %w", mount.chroot, err)
			}
			file.Close()
		}
		out, err := exec.Command("mount", "--bind", mount.host, target).CombinedOutput()
		if err != nil {
			return fmt.Errorf("mounting %s into the chroot: %v: %s", mount.chroot, err, out)
		}
	}
	return nil
}

func unmountChroot(flavor string, job *jobs.Job) {
	dir := chrootDir(flavor)
	for _, mount := range chrootMounts {
		target := filepath.Join(dir, mount.chroot)
		out, err := exec.Command("umount", target).CombinedOutput()
		if err != nil && !strings.Contains(string(out), "not mounted") {
			job.WriteProgress("warning: couldn't unmount " + mount.chroot + ": " + string(out))
		}
	}
}

const repoRefresh = 7 * 24 * time.Hour

func repoMarkerPath(flavor string) string {
	return filepath.Join(chrootDir(flavor), "var/db/repos/gentoo/.eserved-repo-updated")
}

func chrootRepoDir(flavor string) string {
	return filepath.Join(chrootDir(flavor), "var/db/repos/gentoo")
}

// bwrap wraps the chroot in fresh pid/ipc/uts/mount namespaces
func useBwrap() bool {
	_, err := exec.LookPath("bwrap")
	return err == nil
}

func flavorCommand(ctx context.Context, job *jobs.Job, flavor, exe string, args ...string) *exec.Cmd {
	inner := append([]string{
		"/usr/bin/env", "-i",
		"PATH=/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root",
		exe,
	}, args...)
	bwrap := useBwrap()
	var command *exec.Cmd
	if bwrap {
		full := append([]string{
			"--new-session", "--die-with-parent",
			"--unshare-pid", "--unshare-ipc", "--unshare-uts",
			"--bind", chrootDir(flavor), "/",
			"--dev-bind", "/dev", "/dev",
			"--proc", "/proc",
			"--bind", "/sys", "/sys",
			"--bind", "/etc/resolv.conf", "/etc/resolv.conf",
			"--bind", "/etc/hosts", "/etc/hosts",
			"--chdir", "/",
			"--",
		}, inner...)
		command = exec.CommandContext(ctx, "bwrap", full...)
	} else {
		full := append([]string{chrootDir(flavor)}, inner...)
		command = exec.CommandContext(ctx, "chroot", full...)
	}
	command.Stdout = &jobs.JobWriter{Job: job}
	command.Stderr = &jobs.JobWriter{Job: job}
	return command
}

func flavorEmerge(ctx context.Context, job *jobs.Job, flavor string, args ...string) *exec.Cmd {
	return flavorCommand(ctx, job, flavor, "/usr/bin/emerge", args...)
}

// fresh marker? as-is. stale? sync. sync fails? copy the server's own repo
func ensureRepo(ctx context.Context, job *jobs.Job, flavor string) error {
	repoDir := chrootRepoDir(flavor)
	marker := repoMarkerPath(flavor)

	if info, err := os.Stat(marker); err == nil && time.Since(info.ModTime()) < repoRefresh {
		job.WriteProgress("the chroot's gentoo repo is fresh, skipping the sync")
		return nil
	}

	if _, err := os.Stat(repoDir); err == nil {
		job.WriteProgress("syncing the chroot's gentoo repo")
		if err := flavorEmerge(ctx, job, flavor, "--sync").Run(); err == nil {
			return touchRepoMarker(job, flavor)
		}
		job.WriteProgress("portage sync failed, falling back to the server's repo")
	}

	hostRepo := hostRepoDir
	if _, err := os.Stat(hostRepo); err != nil {
		return fmt.Errorf("the chroot repo sync failed and the server has no %s to copy: %w", hostRepo, err)
	}

	job.WriteProgress("copying the server's gentoo repo into the chroot")
	if err := os.RemoveAll(repoDir); err != nil {
		return fmt.Errorf("couldn't remove the stale chroot repo: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(repoDir), 0o755); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "cp", "-a", hostRepo, repoDir)
	command.Stdout = &jobs.JobWriter{Job: job}
	command.Stderr = &jobs.JobWriter{Job: job}
	if err := command.Run(); err != nil {
		return fmt.Errorf("copying the server's repo: %w", err)
	}
	return touchRepoMarker(job, flavor)
}

func touchRepoMarker(job *jobs.Job, flavor string) error {
	job.WriteProgress("the chroot's gentoo repo is up to date")
	return os.WriteFile(repoMarkerPath(flavor), nil, 0o644)
}

func setupChrootSigning(flavor string) error {
	if err := gpg.CopyTo(filepath.Join(chrootDir(flavor), "etc/eserved-gnupg")); err != nil {
		return fmt.Errorf("couldn't copy the signing key into the chroot: %w", err)
	}
	fingerprint, err := gpg.KeyFingerprint()
	if err != nil {
		return err
	}
	// the flavor layer wins make.conf, so the key id gets appended, never layered
	return upsertMakeConfLine(filepath.Join(chrootDir(flavor), "etc/portage/make.conf"), "BINPKG_GPG_SIGNING_KEY", fmt.Sprintf(`"%s"`, fingerprint))
}

func upsertMakeConfLine(path, variable, value string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out := make([]string, 0, len(strings.Split(string(data), "\n"))+1)
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, variable+"=") {
			line = variable + "=" + value
			found = true
		}
		out = append(out, line)
	}
	if !found {
		out = append(out, variable+"="+value)
	}
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
}

func cleanAtomCache(flavor string, atoms []string) {
	for _, atom := range atoms {
		dir, err := atomPkgDir(flavor, atom)
		if err != nil {
			continue // validated before the build, nothing to clean
		}
		os.RemoveAll(dir)
	}
}

func checkBuiltPkgs(flavor string, atoms []string) error {
	for _, atom := range atoms {
		dir, err := atomPkgDir(flavor, atom)
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("no binpkg was produced for %s: %w", atom, err)
		}
		ok := false
		for _, e := range entries {
			info, err := e.Info()
			if err == nil && !e.IsDir() && strings.HasSuffix(e.Name(), ".gpkg.tar") && info.Size() > 0 {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("no usable binpkg was produced for %s", atom)
		}
	}
	return nil
}

// a client sync can swap the portage config mid-build, so take the flavor lock too
func BuildJob(ctx context.Context, job *jobs.Job, flavor string, packages []string) error {
	if !ValidFlavor(flavor) {
		return fmt.Errorf("invalid flavor %q", flavor)
	}
	if len(packages) == 0 {
		return fmt.Errorf("no packages to build")
	}
	for _, atom := range packages {
		if err := validateBuildAtom(atom); err != nil {
			return err
		}
	}
	// a broken cross.conf must not quietly turn into a native build
	crossTarget, crossErr := CrossTargetError(flavor)
	if crossErr != nil {
		return crossErr
	}
	hasCross := crossTarget != ""
	if !IsProvisioned(flavor) {
		return fmt.Errorf("flavor %q is not provisioned", flavor)
	}

	unlock := flavorlock.Lock(flavor)
	defer unlock()

	// a fresh flavor whose layer never landed would build unsigned gpkgs, so make
	// sure it is in the chroot before anything reads the portage config
	if !flavorLayerApplied(flavor) {
		job.WriteProgress("applying the flavor config")
		if err := applyFlavorToChrootLocked(ctx, flavor, ClientSyncArchives(flavor), ""); err != nil {
			return err
		}
	}
	if !hasFlavorMakeConf(flavor) {
		job.WriteProgress("warning: flavor " + flavor + " has no make.conf, the gpkgs will be unsigned")
	}

	job.WriteProgress("preparing the chroot")
	if !useBwrap() {
		if err := mountChroot(flavor); err != nil {
			return err
		}
		defer unmountChroot(flavor, job)
	}
	if err := ensureRepo(ctx, job, flavor); err != nil {
		return err
	}

	if err := setupChrootSigning(flavor); err != nil {
		return err
	}

	if hasCross {
		if err := ensureCrossDev(ctx, job, flavor, crossTarget); err != nil {
			return err
		}
		if err := setupCrossSysroot(flavor, crossTarget); err != nil {
			return err
		}
	}

	// stale cached gpkgs of the requested atoms would get published next to the fresh ones
	cleanAtomCache(flavor, packages)

	threads := serverConfig.Settings.BuildThreads
	parallel := "-j" // 0 = unlimited, let portage decide
	if threads > 0 {
		parallel = fmt.Sprintf("-j%d", threads)
	}

	// the atoms go after -- so a stray flag can never be read as an option; --update
	// so a plain cat/pkg builds the newest visible version (what clients upgrade to)
	// instead of rebuilding whatever the chroot happens to have installed, and
	// --selective=n so an already-current package is still rebuilt into a binpkg
	args := append([]string{"--buildpkg", "--usepkg=n", "--getbinpkg=n", "--update", "--selective=n", parallel, "--"}, packages...)

	if hasCross {
		// the sysroot wrapper emerges into /usr/<target> with the target CHOST and
		// leaves the gpkgs in the sysroot's PKGDIR
		job.WriteProgress("cross-building " + strings.Join(packages, ", ") + " for " + crossTarget)
		if err := flavorCommand(ctx, job, flavor, "/usr/bin/emerge-"+crossTarget, args...).Run(); err != nil {
			return fmt.Errorf("cross emerge failed: %w", err)
		}
	} else {
		job.WriteProgress("building " + strings.Join(packages, ", "))
		if err := flavorEmerge(ctx, job, flavor, args...).Run(); err != nil {
			return fmt.Errorf("emerge failed: %w", err)
		}
	}

	// portage can exit 0 even when the build produced nothing usable
	if err := checkBuiltPkgs(flavor, packages); err != nil {
		return err
	}

	job.WriteProgress("publishing the binpkgs to the binhost")
	return PublishBinpkgs(job, flavor)
}
