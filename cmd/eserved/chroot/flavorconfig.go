package chroot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/config"
	"git.fedesito.me/fedes1to/eserve/internal/flavorlock"
)

func hasFlavorConfig(flavor string) bool {
	entries, err := os.ReadDir(config.FlavorConfigDir(flavor))
	if err != nil {
		return false
	}
	for _, entry := range entries {
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
		return nil, false, nil // no flavor config yet, not an error
	}

	present = make(map[string]bool)
	for _, entry := range entries {
		if !syncedPathSet[entry.Name()] {
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

// the last archive wins, same as the last sync did
func ApplyFlavorToChroot(ctx context.Context, flavor string, archives []string) error {
	if !IsProvisioned(flavor) {
		return fmt.Errorf("flavor %q is not provisioned", flavor)
	}
	unlock := flavorlock.Lock(flavor)
	defer unlock()

	root, err := os.OpenRoot(chrootDir(flavor))
	if err != nil {
		return fmt.Errorf("couldn't open chroot: %w", err)
	}
	defer root.Close()

	sweepStaleConfigDirs(root, chrootDir(flavor))

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
	return nil
}

func ClientSyncArchives(flavor string) []string {
	archives, _ := filepath.Glob(filepath.Join(serverConfig.ServerConfigPath, "sync", flavor, "*.tar.gz"))
	sort.Strings(archives)
	return archives
}
