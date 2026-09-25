package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/chroot"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/config"
)

type FlavorFingerprint struct {
	Fingerprint string    `json:"fingerprint"`
	SyncedBy    string    `json:"synced_by"`
	SyncedAt    time.Time `json:"synced_at"`
}

func flavorFingerprintPath(flavor string) string {
	return filepath.Join(serverConfig.ServerConfigPath, "sync", flavor, "fingerprint.json")
}

// the whole fingerprint record (who synced the flavor and when, not
// just the fingerprint itself)
func FlavorFingerprintInfo(flavor string) (fp FlavorFingerprint, ok bool) {
	if err := config.LoadJsonFile(flavorFingerprintPath(flavor), &fp); err != nil {
		return fp, false
	}
	return fp, fp.Fingerprint != ""
}

func GetFlavorFingerprint(flavor string) (fingerprint string, ok bool) {
	fp, ok := FlavorFingerprintInfo(flavor)
	return fp.Fingerprint, ok
}

func SetFlavorFingerprint(flavor, fingerprint, syncedBy string) error {
	syncDir := filepath.Join(serverConfig.ServerConfigPath, "sync", flavor)
	if err := os.MkdirAll(syncDir, 0700); err != nil {
		return err
	}
	return config.SafeSaveJsonFile(flavorFingerprintPath(flavor), FlavorFingerprint{
		Fingerprint: fingerprint,
		SyncedBy:    syncedBy,
		SyncedAt:    time.Now(),
	})
}

// the roots the flavor-existence check looks at, vars so tests can point them at temp dirs
var (
	flavorChrootRoot = func() string { return serverConfig.Settings.ChrootBase }
	flavorConfigRoot = func() string { return config.ServerConfigPath }
)

// a flavor exists once it has a provisioned chroot, a machine on it, or a config
// dir of its own. reads the machines map, so callers hold machinesMutex
func flavorExistsIn(chrootBase, configBase, flavor string) bool {
	if !chroot.ValidFlavor(flavor) {
		return false
	}
	if chroot.IsProvisionedIn(chrootBase, flavor) {
		return true
	}
	if _, err := os.Stat(filepath.Join(configBase, "flavors", flavor)); err == nil {
		return true
	}
	for _, entry := range machines.Entries {
		if entry.Flavor == flavor {
			return true
		}
	}
	return false
}

// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func flavorExistsLocked(flavor string) bool {
	return flavorExistsIn(flavorChrootRoot(), flavorConfigRoot(), flavor)
}

// joining a flavor that already exists needs a token bound to it: an unbound
// token may only start a brand new flavor, whose fresh chroot nobody else is on.
// a machine already on the flavor is recovering, not joining
// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func flavorJoinRefusalLocked(tokenFlavor, cn, flavor string) error {
	if tokenFlavor != "" || !flavorExistsLocked(flavor) {
		return nil
	}
	if entry, ok := machines.Entries[cn]; ok && entry.Flavor == flavor {
		return nil
	}
	return fmt.Errorf("%w: flavor %s already exists, use a token bound to it (eservectl token create -flavor %s)", ErrFlavorExists, flavor, flavor)
}

// the distinct profiles of the machines on a flavor, sorted
func FlavorProfiles(flavor string) []string {
	machinesMutex.RLock()
	defer machinesMutex.RUnlock()

	seen := make(map[string]bool)
	profiles := make([]string, 0, len(machines.Entries))
	for _, entry := range machines.Entries {
		if entry.Flavor != flavor || entry.Profile.Full == "" || seen[entry.Profile.Full] {
			continue
		}
		seen[entry.Profile.Full] = true
		profiles = append(profiles, entry.Profile.Full)
	}
	slices.Sort(profiles)
	return profiles
}

// the profile the machine last provisioned with
func MachineProfile(cn string) (string, bool) {
	machinesMutex.RLock()
	defer machinesMutex.RUnlock()

	entry, exists := machines.Entries[cn]
	return entry.Profile.Full, exists
}
