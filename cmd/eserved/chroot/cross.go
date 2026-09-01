package chroot

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/jobs"
	"git.fedesito.me/fedes1to/eserve/internal/config"
)

// cross.conf in the flavor dir names the target triple this flavor cross-builds for
func CrossTarget(flavor string) (string, bool) {
	f, err := os.Open(filepath.Join(config.FlavorConfigDir(flavor), "cross.conf"))
	if err != nil {
		return "", false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		if v, ok := strings.CutPrefix(line, "target="); ok {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

// a flavor with a cross target also serves that target's arch, not just the server's
func CrossCoversArch(flavor, clientGccMachine string) bool {
	target, ok := CrossTarget(flavor)
	if !ok || clientGccMachine == "" {
		return false
	}
	clientArch, _, _ := strings.Cut(clientGccMachine, "-")
	targetArch, _, _ := strings.Cut(target, "-")
	return clientArch == targetArch
}

// the cross dev sdk marker, same convention as the repo marker
func crossSdkMarker(flavor, target string) string {
	return filepath.Join(chrootDir(flavor), "eserved-cross-"+target)
}

// make sure the crossdev tool and the target sdk are present in the chroot
func ensureCrossDev(ctx context.Context, job *jobs.Job, flavor, target string) error {
	if info, err := os.Stat(crossSdkMarker(flavor, target)); err == nil && info.ModTime().Add(repoRefresh).After(time.Now()) {
		return nil
	}

	job.WriteProgress("installing crossdev in the chroot")
	if err := flavorEmerge(ctx, job, flavor, "-N", "--usepkg=n", "--getbinpkg=n", "sys-devel/crossdev").Run(); err != nil {
		return fmt.Errorf("installing crossdev: %w", err)
	}
	overlayParent := filepath.Join(chrootDir(flavor), "usr/portage/local/crossdev")
	if err := os.MkdirAll(overlayParent, 0o755); err != nil {
		return fmt.Errorf("preparing the crossdev overlay: %w", err)
	}
	// stage3 chroots ship /etc/portage without subdirs, crossdev writes repos.conf/crossdev.conf
	if err := os.MkdirAll(filepath.Join(chrootDir(flavor), "etc/portage/repos.conf"), 0o755); err != nil {
		return fmt.Errorf("preparing the crossdev repo conf: %w", err)
	}
	job.WriteProgress("building the cross dev sdk for " + target)
	if err := flavorCommand(ctx, job, flavor, "/usr/bin/crossdev", "-t", target, "-oO", "/usr/portage/local/crossdev", "--portage", "-v").Run(); err != nil {
		return fmt.Errorf("cross dev sdk build failed: %w", err)
	}

	return os.WriteFile(crossSdkMarker(flavor, target), nil, 0o644)
}

// flavor dir crossdev/** gets mirrored into the chroot's crossdev repo:
// the user's territory, same as the use-flag fixes
func SyncCrossOverlay(flavor, target string) error {
	src := filepath.Join(config.FlavorConfigDir(flavor), "crossdev")
	dst := filepath.Join("usr/portage/local/crossdev", "cross-"+target)
	root, err := os.OpenRoot(chrootDir(flavor))
	if err != nil {
		return fmt.Errorf("couldn't open the chroot: %w", err)
	}

	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil // no crossdev dir yet, nothing to sync
			}
			return err
		}
		if p == src {
			return nil
		}
		rel, relErr := filepath.Rel(src, p)
		if relErr != nil || strings.Contains(rel, "..") {
			return fmt.Errorf("path traversal %q in the flavor crossdev dir", rel)
		}
		full := filepath.Join(dst, rel)
		if d.IsDir() {
			return root.MkdirAll(full, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return root.WriteFile(full, data, 0o644)
	})
	if err != nil {
		return fmt.Errorf("syncing the crossdev overlay: %w", err)
	}
	return nil
}

// a cross ebuild for plain packages has to come from the user's flavor overlay
func CrossEbuildPath(flavor, target, catPn string) string {
	return filepath.Join(chrootDir(flavor), "usr/portage/local/crossdev", "cross-"+target, catPn)
}

func crossEbuildPresent(flavor, target, catPn string) bool {
	entries, err := os.ReadDir(CrossEbuildPath(flavor, target, catPn))
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".ebuild") {
			return true
		}
	}
	return false
}

// a plain cat/pkg for crossdev, dropping any version a user typed in
func crossAtom(atom string) string {
	parts := strings.SplitN(atom, "/", 2)
	if len(parts) < 2 {
		return atom
	}
	name := parts[1]
	if i := strings.Index(name, "-"); i > 0 {
		name = name[:i]
	}
	return parts[0] + "/" + name
}
