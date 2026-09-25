package chroot

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/jobs"
	"git.fedesito.me/fedes1to/eserve/internal/config"
	"git.fedesito.me/fedes1to/eserve/internal/gpg"
)

// the target ends up in paths and in the emerge-<target> wrapper name
var crossTargetPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// cross.conf in the flavor dir names the target triple this flavor cross-builds for.
// no cross.conf means "not a cross flavor"; a cross.conf with a bad target is an error,
// never a silent fallback to a native build
func CrossTargetError(flavor string) (string, error) {
	f, err := os.Open(filepath.Join(config.FlavorConfigDir(flavor), "cross.conf"))
	if err != nil {
		return "", nil
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		if v, ok := strings.CutPrefix(line, "target="); ok {
			v = strings.TrimSpace(v)
			if !crossTargetPattern.MatchString(v) {
				return "", fmt.Errorf("invalid cross target %q in flavors/%s/cross.conf", v, flavor)
			}
			return v, nil
		}
	}
	return "", fmt.Errorf("flavors/%s/cross.conf has no target= line", flavor)
}

// a flavor with a valid cross target
func CrossTarget(flavor string) (string, bool) {
	target, err := CrossTargetError(flavor)
	return target, err == nil && target != ""
}

// why a client can't provision into this flavor, "" when it can. a cross flavor
// serves exactly its target's CHOST (a client with a different triple can't
// install the binpkgs it builds), a native one serves the server's own CHOST —
// same arch and same os/libc, the vendor doesn't matter
func ArchRefusal(flavor, clientGccMachine string) string {
	if target, ok := CrossTarget(flavor); ok {
		if clientGccMachine != "" && target != clientGccMachine {
			return fmt.Sprintf("flavor %s cross-builds for %s, this machine is %s", flavor, target, clientGccMachine)
		}
		return ""
	}
	if clientGccMachine == "" || serverGccMachine == "" {
		return ""
	}
	clientArch, clientTail := chostTarget(clientGccMachine)
	serverArch, serverTail := chostTarget(serverGccMachine)
	if clientArch == serverArch && clientTail == serverTail {
		return ""
	}
	return fmt.Sprintf("flavor %s builds for %s, this machine is %s; use a cross flavor for %s", flavor, serverGccMachine, clientGccMachine, clientGccMachine)
}

// the cross dev sdk marker, same convention as the repo marker
func crossSdkMarker(flavor, target string) string {
	return filepath.Join(chrootDir(flavor), "eserved-cross-"+target)
}

// the sysroot crossdev installs the target toolchain into
func crossSysrootDir(flavor, target string) string {
	return filepath.Join(chrootDir(flavor), "usr", target)
}

// target binpkgs land in the sysroot's PKGDIR, not in the chroot's
func crossBinpkgDir(flavor, target string) string {
	return filepath.Join(crossSysrootDir(flavor, target), "var/cache/binpkgs")
}

// where a flavor's build jobs leave their binpkgs
func binpkgDir(flavor string) string {
	if target, ok := CrossTarget(flavor); ok {
		return crossBinpkgDir(flavor, target)
	}
	return filepath.Join(chrootDir(flavor), "var/cache/binpkgs")
}

// make sure the crossdev tool and the target sdk are present in the chroot
func ensureCrossDev(ctx context.Context, job *jobs.Job, flavor, target string) error {
	if info, err := os.Stat(crossSdkMarker(flavor, target)); err == nil && info.ModTime().Add(repoRefresh).After(time.Now()) {
		if _, err := os.Stat(filepath.Join(crossSysrootDir(flavor, target), "etc/portage/make.conf")); err == nil {
			return nil
		}
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

const (
	sysrootBlockStart = "# eserved flavor layer"
	sysrootBlockEnd   = "# end eserved flavor layer"
)

// the sysroot is a portage config root of its own, so it needs the flavor's build env too
func setupCrossSysroot(flavor, target string) error {
	portageDir := filepath.Join(crossSysrootDir(flavor, target), "etc/portage")
	// crossdev writes this when it first sets the sysroot up, a wiped one would break the build
	profile := filepath.Join(portageDir, "make.profile")
	// the sysroot's profile: the flavor's override wins (a cross target usually wants
	// its own arch profile, e.g. default/linux/amd64/23.0/musl, or the binpkgs come
	// out with the host's ABI flags), otherwise crossdev's embedded one
	sysrootProfile := "/var/db/repos/gentoo/profiles/embedded"
	if override, hasOverride, err := flavorProfileOverride(flavor); err != nil {
		return err
	} else if hasOverride {
		if info, err := os.Stat(filepath.Join(chrootDir(flavor), gentooProfilesDir, override)); err != nil || !info.IsDir() {
			return fmt.Errorf("profile %q is not in the chroot's %s", override, gentooProfilesDir)
		}
		sysrootProfile = "/var/db/repos/gentoo/profiles/" + override
	}
	if current, err := os.Readlink(profile); err != nil || current != sysrootProfile {
		if err := os.Remove(profile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("couldn't replace the sysroot profile: %w", err)
		}
		if err := os.Symlink(sysrootProfile, profile); err != nil {
			return fmt.Errorf("couldn't set the sysroot profile: %w", err)
		}
	}

	path := filepath.Join(portageDir, "make.conf")
	base, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("no sysroot make.conf for %s: %w", target, err)
	}
	flavorConf, err := os.ReadFile(filepath.Join(config.FlavorConfigDir(flavor), "make.conf"))
	if err != nil {
		return fmt.Errorf("couldn't read the flavor make.conf: %w", err)
	}
	fingerprint, err := gpg.KeyFingerprint()
	if err != nil {
		return err
	}

	// the flavor layer is appended, so its FEATURES="${FEATURES} ..." still appends
	var kept []string
	inBlock := false
	for _, line := range strings.Split(string(base), "\n") {
		switch line {
		case sysrootBlockStart:
			inBlock = true
		case sysrootBlockEnd:
			inBlock = false
		default:
			if !inBlock {
				kept = append(kept, line)
			}
		}
	}
	block := []string{
		sysrootBlockStart,
		strings.TrimRight(string(flavorConf), "\n"),
		fmt.Sprintf("BINPKG_GPG_SIGNING_KEY=%q", fingerprint),
		sysrootBlockEnd,
	}
	return os.WriteFile(path, []byte(strings.Join(append(kept, block...), "\n")), 0o644)
}
