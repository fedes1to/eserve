package chroot

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/jobs"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/config"
	"git.fedesito.me/fedes1to/eserve/internal/flavorlock"
)

// flavors/<name>/profile as a regular file is the chroot profile override, not
// the portage user profile (that one is a directory)
func isProfileOverride(entry os.DirEntry) bool {
	return entry.Name() == "profile" && !entry.IsDir()
}

func hasFlavorConfig(flavor string) bool {
	entries, err := os.ReadDir(config.FlavorConfigDir(flavor))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if isProfileOverride(entry) {
			continue
		}
		if syncedPathSet[entry.Name()] {
			return true
		}
	}
	return false
}

// the client's archive lands over these; the flavor's make.conf is re-applied last
func copyFlavorConfig(root *os.Root, staging, flavor string) (present map[string]bool, hasMakeConf bool, err error) {
	base := config.FlavorConfigDir(flavor)
	entries, err := os.ReadDir(base)
	if err != nil {
		return map[string]bool{}, false, nil // no flavor config yet, not an error
	}

	present = make(map[string]bool)
	for _, entry := range entries {
		if !syncedPathSet[entry.Name()] || isProfileOverride(entry) {
			continue
		}
		full := filepath.Join(base, entry.Name())
		if entry.IsDir() {
			// a .d directory: take the regular files one level down
			children, err := os.ReadDir(full)
			if err != nil {
				return nil, false, err
			}
			for _, child := range children {
				if child.IsDir() {
					continue
				}
				present, hasMakeConf, err = copyFlavorFile(root, staging, base, filepath.Join(entry.Name(), child.Name()), present, hasMakeConf)
				if err != nil {
					return nil, false, err
				}
			}
			present[entry.Name()] = true
			continue
		}
		present, hasMakeConf, err = copyFlavorFile(root, staging, base, entry.Name(), present, hasMakeConf)
		if err != nil {
			return nil, false, err
		}
	}
	return present, hasMakeConf, nil
}

func copyFlavorFile(root *os.Root, staging, base, rel string, present map[string]bool, hasMakeConf bool) (map[string]bool, bool, error) {
	if strings.Contains(rel, "..") {
		return nil, false, fmt.Errorf("path traversal %q in flavor config", rel)
	}
	top, _, _ := strings.Cut(rel, "/")
	if !syncedPathSet[top] {
		return present, hasMakeConf, nil
	}

	data, err := os.ReadFile(filepath.Join(base, rel))
	if err != nil {
		return nil, false, fmt.Errorf("couldn't read flavor config file %q: %w", rel, err)
	}
	if err := root.MkdirAll(staging+"/"+filepath.Dir(rel), 0o755); err != nil {
		return nil, false, err
	}
	if err := root.WriteFile(staging+"/"+rel, data, 0o644); err != nil {
		return nil, false, err
	}

	present[top] = true
	if rel == "make.conf" {
		hasMakeConf = true
	}
	return present, hasMakeConf, nil
}

func overrideFlavorMakeConf(root *os.Root, staging, flavor string, hasMakeConf bool) error {
	if !hasMakeConf {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(config.FlavorConfigDir(flavor), "make.conf"))
	if err != nil {
		return fmt.Errorf("couldn't read flavor make.conf: %w", err)
	}
	return root.WriteFile(staging+"/make.conf", data, 0o644)
}

func buildStagedConfig(ctx context.Context, root *os.Root, staging, flavor, clientArchive string) (present map[string]bool, fingerprint string, err error) {
	if err := root.MkdirAll(staging, 0o755); err != nil {
		return nil, "", err
	}

	present, hasMakeConf, err := copyFlavorConfig(root, staging, flavor)
	if err != nil {
		return nil, "", err
	}

	if clientArchive != "" {
		var archivePresent map[string]bool
		archivePresent, fingerprint, err = extractArchive(ctx, clientArchive, root, staging)
		if err != nil {
			return nil, "", err
		}
		for top := range archivePresent {
			present[top] = true
		}
	}

	if err := overrideFlavorMakeConf(root, staging, flavor, hasMakeConf); err != nil {
		return nil, "", err
	}
	return present, fingerprint, nil
}

// the atomic swap that makes the etc/portage links never dangle
func installStagedConfig(root *os.Root, staging string, present map[string]bool) error {
	suffix, err := randomSuffix()
	if err != nil {
		return err
	}
	trash := ConfigDir + ".old-" + suffix
	_, statErr := root.Lstat(ConfigDir)
	hadOld := statErr == nil
	if hadOld {
		if err := root.Rename(ConfigDir, trash); err != nil {
			root.RemoveAll(staging)
			return fmt.Errorf("couldn't set aside old config: %w", err)
		}
	}
	if err := root.Rename(staging, ConfigDir); err != nil {
		if hadOld {
			root.Rename(trash, ConfigDir)
		}
		root.RemoveAll(staging)
		return fmt.Errorf("couldn't install new config: %w", err)
	}
	if hadOld {
		if err := root.RemoveAll(trash); err != nil {
			return fmt.Errorf("couldn't remove old config: %w", err)
		}
	}
	return linkSyncedPaths(root, present)
}

// the last archive wins, same as the last sync did; profile is the last syncing
// machine's, and the chroot's make.profile follows it
func ApplyFlavorToChroot(ctx context.Context, flavor string, archives []string, profile string) error {
	if !IsProvisioned(flavor) {
		return fmt.Errorf("flavor %q is not provisioned", flavor)
	}
	unlock := flavorlock.Lock(flavor)
	defer unlock()
	return applyFlavorToChrootLocked(ctx, flavor, archives, profile)
}

// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func applyFlavorToChrootLocked(ctx context.Context, flavor string, archives []string, profile string) error {
	root, err := os.OpenRoot(chrootDir(flavor))
	if err != nil {
		return fmt.Errorf("couldn't open chroot: %w", err)
	}
	defer root.Close()

	sweepStaleConfigDirs(root, chrootDir(flavor))

	// a flavor nobody has synced yet still needs its own layer installed, so an
	// empty archive list means "the flavor layer alone"
	if len(archives) == 0 && hasFlavorConfig(flavor) {
		archives = []string{""}
	}

	for _, archive := range archives {
		suffix, err := randomSuffix()
		if err != nil {
			return err
		}
		staging := ConfigDir + ".new-" + suffix
		present, _, err := buildStagedConfig(ctx, root, staging, flavor, archive)
		if err != nil {
			root.RemoveAll(staging)
			return err
		}
		if err := installStagedConfig(root, staging, present); err != nil {
			return err
		}
	}
	// the profile of the machine whose config ended up on top
	return setChrootProfileLocked(flavor, profile)
}

func hasFlavorMakeConf(flavor string) bool {
	info, err := os.Stat(filepath.Join(config.FlavorConfigDir(flavor), "make.conf"))
	return err == nil && info.Mode().IsRegular()
}

// a crashed build can leave bind mounts behind, and removing the tree under them
// would be a mess
func activeChrootMounts(chrootDir string) []string {
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil
	}
	defer file.Close()

	var mounts []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// mountinfo: the mountpoint is field 5, before the " - "
		parts := strings.SplitN(scanner.Text(), " - ", 2)
		if len(parts) != 2 {
			continue
		}
		fields := strings.Fields(parts[0])
		if len(fields) >= 5 && strings.HasPrefix(fields[4], chrootDir+string(filepath.Separator)) {
			mounts = append(mounts, fields[4])
		}
	}
	return mounts
}

// a flavor goes away entirely: its chroot, its binhost, its sync archives and its
// config dir. refuses while a job is queued or running for it
func DeleteFlavor(flavor string) error {
	if !ValidFlavor(flavor) {
		return fmt.Errorf("invalid flavor %q", flavor)
	}
	busy := func() error {
		if jobs.Registry.FlavorBusy(flavor) {
			return fmt.Errorf("flavor %s has a job queued or running, cancel it first", flavor)
		}
		return nil
	}
	// before the lock: a running job holds it, and waiting for the job to finish is
	// not what "delete this flavor" should do
	if err := busy(); err != nil {
		return err
	}
	unlock := flavorlock.Lock(flavor)
	defer unlock()
	// and again now that nothing can be running
	if err := busy(); err != nil {
		return err
	}
	if mounts := activeChrootMounts(chrootDir(flavor)); len(mounts) > 0 {
		return fmt.Errorf("flavor %s still has mounts under its chroot (%s), unmount them first", flavor, strings.Join(mounts, ", "))
	}

	for _, path := range []string{
		chrootDir(flavor),
		filepath.Join(serverConfig.Settings.RepoBase, flavor),
		filepath.Join(serverConfig.ServerConfigPath, "sync", flavor),
		config.FlavorConfigDir(flavor),
	} {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("couldn't remove %s: %w", path, err)
		}
	}
	return nil
}

// the flavor's make.conf carries the signing config, so the chroot's make.conf
// being our link into .eserved/ is what "the flavor layer landed" means
func flavorLayerApplied(flavor string) bool {
	if !hasFlavorMakeConf(flavor) {
		return true // nothing to apply
	}
	target, err := os.Readlink(filepath.Join(chrootDir(flavor), "etc/portage/make.conf"))
	return err == nil && target == "../../"+ConfigDir+"/make.conf"
}

func ClientSyncArchives(flavor string) []string {
	archives, _ := filepath.Glob(filepath.Join(serverConfig.ServerConfigPath, "sync", flavor, "*.tar.gz"))
	sort.Strings(archives)
	return archives
}
