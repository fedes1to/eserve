package storage

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/chroot"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/config"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

type MachineEntry struct {
	Arch        string         `json:"arch"`
	Subarch     string         `json:"march"`
	Profile     chroot.Profile `json:"profile"`
	Flavor      string         `json:"flavor"`
	Fingerprint string         `json:"fingerprint"`
	RevokedAt   time.Time      `json:"revoked_at"` // IsZero if not revoked
}

// we still address by tokens here, thats the GUID we will use internally
type MachinesFile struct {
	Entries map[string]MachineEntry `json:"machines"`
}

var (
	machines      MachinesFile
	machinesMutex sync.RWMutex
	machinesPath  = filepath.Join(serverConfig.ServerConfigPath, "machines.json")
)

func LoadMachines() error {
	machinesMutex.Lock()
	defer machinesMutex.Unlock()
	if err := loadMachinesLocked(); err != nil {
		return err
	}
	return nil
}

// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func saveMachinesLocked() error {
	if err := config.SafeSaveJsonFile(machinesPath, machines); err != nil {
		return err
	}
	return nil
}

// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func loadMachinesLocked() error {
	if err := config.LoadJsonFile(machinesPath, &machines); err != nil {
		return err
	}

	if machines.Entries == nil {
		machines.Entries = make(map[string]MachineEntry)
	}

	return nil
}

func RevokeMachine(cn string) error {
	machinesMutex.Lock()
	defer machinesMutex.Unlock()

	machineEntry, exists := machines.Entries[cn]
	if !exists {
		return fmt.Errorf("Can't revoke non-existant machine %v", cn)
	}

	machineEntry.RevokedAt = time.Now()
	machines.Entries[cn] = machineEntry

	return saveMachinesLocked()
}

func ProvisionMachine(cn, subarch, gccMachine, profile, flavor string) error {
	machinesMutex.Lock()
	defer machinesMutex.Unlock()

	upsertedEntry, exists := machines.Entries[cn]
	if !exists || upsertedEntry.Fingerprint == "" {
		return fmt.Errorf("machine %v has no certificate — run identity first", cn)
	}

	if chroot.IsGccMachineDiff(gccMachine) {
		return fmt.Errorf("Crossdev support not implemented, choose same arch as eserved")
	}

	entry := MachineEntry{
		Subarch:     subarch,
		Profile:     chroot.Profile{Full: profile, GccMachine: gccMachine},
		Flavor:      flavor,
		Fingerprint: upsertedEntry.Fingerprint,
		RevokedAt:   upsertedEntry.RevokedAt, // revocation is sticky, a re-provision doesnt clear it
	}
	machines.Entries[cn] = entry
	return saveMachinesLocked()
}

func MachineFlavor(cn string) (string, bool) {
	machinesMutex.RLock()
	defer machinesMutex.RUnlock()

	entry, exists := machines.Entries[cn]
	return entry.Flavor, exists
}

func MachineExists(cn string) bool {
	machinesMutex.RLock()
	defer machinesMutex.RUnlock()

	_, exists := machines.Entries[cn]
	return exists
}

func IsMachineCrossdev(cn string) bool {
	machinesMutex.RLock()
	defer machinesMutex.RUnlock()

	return machines.Entries[cn].Profile.IsCrossdev()
}

func UpsertMachine(cn, fingerprint, flavor string) error {
	machinesMutex.Lock()
	defer machinesMutex.Unlock()

	entry := machines.Entries[cn]
	entry.Fingerprint = fingerprint
	entry.Flavor = flavor
	machines.Entries[cn] = entry
	return saveMachinesLocked()
}

func MachineCertValid(cn, fingerprint string) bool {
	machinesMutex.RLock()
	defer machinesMutex.RUnlock()
	entry, exists := machines.Entries[cn]
	if !exists {
		return false
	}
	return entry.Fingerprint != "" && entry.Fingerprint == fingerprint && entry.RevokedAt.IsZero()
}

// returns the registered machines, sorted by cn
func ListMachines() []protocol.MachineInfo {
	machinesMutex.RLock()
	defer machinesMutex.RUnlock()

	list := make([]protocol.MachineInfo, 0, len(machines.Entries))
	for cn, entry := range machines.Entries {
		list = append(list, protocol.MachineInfo{
			CN:          cn,
			Subarch:     entry.Subarch,
			Profile:     entry.Profile.Full,
			Flavor:      entry.Flavor,
			Fingerprint: entry.Fingerprint,
			RevokedAt:   entry.RevokedAt,
		})
	}
	slices.SortFunc(list, func(a, b protocol.MachineInfo) int { return strings.Compare(a.CN, b.CN) })
	return list
}
