package chroot

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"git.fedesito.me/fedes1to/eserve/internal/config"
	"git.fedesito.me/fedes1to/eserve/internal/flavorlock"
	"git.fedesito.me/fedes1to/eserve/internal/sysinfo"
)

var serverGccMachine string

type Profile struct {
	Full       string
	GccMachine string
}

func IsGccMachineDiff(clientGccMachine string) bool {
	if clientGccMachine == "" || serverGccMachine == "" {
		return false // nothing to compare, not a different arch
	}
	clientArch, _, _ := strings.Cut(clientGccMachine, "-")
	serverArch, _, _ := strings.Cut(serverGccMachine, "-")
	return clientArch != serverArch
}

func InitializeGccInfo() error {
	gccMachine, err := sysinfo.GetGccMachine()
	if err != nil {
		return err
	}
	serverGccMachine = gccMachine
	return nil
}

// the gentoo repo's profile tree, in the chroot and on the server
const (
	gentooProfilesDir = "var/db/repos/gentoo/profiles"
	hostRepoDir       = "/var/db/repos/gentoo"
)

// a profile is a relative path under the chroot's profiles dir
func validProfile(profile string) error {
	if profile == "" || strings.HasPrefix(profile, "/") {
		return fmt.Errorf("profile %q must be a relative path", profile)
	}
	for _, component := range strings.Split(profile, "/") {
		if component == "" || component == "." || component == ".." {
			return fmt.Errorf("invalid profile %q", profile)
		}
	}
	return nil
}

// flavors/<name>/profile as a regular file is the chroot profile override; a
// directory of that name is the portage user profile, a synced path like any other
func flavorProfileOverride(flavor string) (string, bool, error) {
	path := filepath.Join(config.FlavorConfigDir(flavor), "profile")
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", false, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		if err := validProfile(line); err != nil {
			return "", false, fmt.Errorf("invalid profile in flavors/%s/profile: %w", flavor, err)
		}
		return line, true, nil
	}
	return "", false, fmt.Errorf("flavors/%s/profile has no profile line", flavor)
}

// the chroot's profile: the admin's flavors/<name>/profile wins, otherwise the
// profile of the machine that last provisioned or synced the flavor
func SetChrootProfile(flavor, profile string) error {
	unlock := flavorlock.Lock(flavor)
	defer unlock()
	return setChrootProfileLocked(flavor, profile)
}

// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func setChrootProfileLocked(flavor, profile string) error {
	override, hasOverride, err := flavorProfileOverride(flavor)
	if err != nil {
		return err
	}
	if hasOverride {
		profile = override
	}
	if profile == "" {
		return nil
	}
	// a cross flavor's sysroot profile is crossdev's, see setupCrossSysroot
	if target, err := CrossTargetError(flavor); err != nil {
		return err
	} else if target != "" {
		return nil
	}
	if err := validProfile(profile); err != nil {
		return err
	}

	root, err := os.OpenRoot(chrootDir(flavor))
	if err != nil {
		return fmt.Errorf("couldn't open chroot: %w", err)
	}
	defer root.Close()

	// a fresh stage3 ships no repo; the first build copies the server's in, so
	// check against that one until the chroot has a repo of its own
	if _, err := root.Stat(gentooProfilesDir); err != nil {
		if info, err := os.Stat(filepath.Join(hostRepoDir, "profiles", profile)); err != nil || !info.IsDir() {
			return fmt.Errorf("profile %q is not in the chroot's %s", profile, gentooProfilesDir)
		}
	} else if info, err := root.Stat(gentooProfilesDir + "/" + profile); err != nil || !info.IsDir() {
		return fmt.Errorf("profile %q is not in the chroot's %s", profile, gentooProfilesDir)
	}

	link := "etc/portage/make.profile"
	if err := root.MkdirAll("etc/portage", 0o755); err != nil {
		return fmt.Errorf("couldn't open etc/portage: %w", err)
	}
	if _, err := root.Lstat(link); err == nil {
		if err := removePath(root, link); err != nil {
			return fmt.Errorf("couldn't replace %s: %w", link, err)
		}
	}
	if err := root.Symlink("../../"+gentooProfilesDir+"/"+profile, link); err != nil {
		return fmt.Errorf("couldn't set %s: %w", link, err)
	}
	return nil
}
