package storage

import (
	"fmt"
	"path/filepath"
	"sync"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/chroot"
	serverConfig "git.fedesito.me/fedes1to/eserve/cmd/eserved/config"
	"git.fedesito.me/fedes1to/eserve/internal/config"
)

type MachineEntry struct {
	Arch        string         `json:"arch"`
	Subarch     string         `json:"march"`
	Profile     chroot.Profile `json:"profile"`
	Flavor      string         `json:"flavor"`
	Fingerprint string         `json:"fingerprint"`
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
	if loadError := loadMachinesLocked(); loadError != nil {
		return loadError
	}
	return nil
}

// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func saveMachinesLocked() error {
	if saveError := config.SafeSaveJsonFile(machinesPath, machines); saveError != nil {
		return saveError
	}
	return nil
}

// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func loadMachinesLocked() error {
	if loadError := config.LoadJsonFile(machinesPath, &machines); loadError != nil {
		return loadError
	}

	if machines.Entries == nil {
		machines.Entries = make(map[string]MachineEntry)
	}

	return nil
}

func ProvisionMachine(cn, subarch, gccMachine, profile, flavor string) error {
	machinesMutex.Lock()
	defer machinesMutex.Unlock()

	upsertedEntry, exists := machines.Entries[cn]
	if !exists || upsertedEntry.Fingerprint == "" {
		return fmt.Errorf("machine %v has no certificate — run identity first", cn)
	}

	if machines.Entries[cn].Profile.IsCrossdev() {
		return fmt.Errorf("Crossdev support not implemented, choose same arch as eserved")
	}

	entry := MachineEntry{
		Subarch:     subarch,
		Profile:     chroot.Profile{Full: profile, GccMachine: gccMachine},
		Flavor:      flavor,
		Fingerprint: upsertedEntry.Fingerprint,
	}
	machines.Entries[cn] = entry
	return saveMachinesLocked()
}

func IsMachineCrossdev(cn string) bool {
	machinesMutex.RLock()
	defer machinesMutex.RUnlock()

	return machines.Entries[cn].Profile.IsCrossdev()
}

func UpsertMachine(cn, fingerprint string) error {
	machinesMutex.Lock()
	defer machinesMutex.Unlock()

	entry := MachineEntry{
		Fingerprint: fingerprint,
	}
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
	return entry.Fingerprint != "" && entry.Fingerprint == fingerprint
}
