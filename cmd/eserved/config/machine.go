package serverConfig

import (
	"fmt"
	"sync"

	"git.fedesito.me/fedes1to/eserve/internal/config"
)

type MachineEntry struct {
	Arch        string `json:"arch"`
	Subarch     string `json:"march"`
	Libc        string `json:"libc"`
	Flavor      string `json:"flavor"`
	Fingerprint string `json:"fingerprint"`
}

// we still address by tokens here, thats the GUID we will use internally
type MachinesFile struct {
	Entries map[string]MachineEntry `json:"machines"`
}

var (
	machines      MachinesFile
	machinesMutex sync.RWMutex
	machinesPath  = ServerConfigPath + "machines.json"
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

func ProvisionMachine(cn, arch, subarch, libc, flavor string) error {
	machinesMutex.Lock()
	defer machinesMutex.Unlock()

	if arch != ServerArch {
		return fmt.Errorf("Architecture mismatch, crossdev support not yet implemented")
	}

	entry := MachineEntry{
		Arch:    arch,
		Subarch: subarch,
		Libc:    libc,
		Flavor:  flavor,
	}
	machines.Entries[cn] = entry
	return saveMachinesLocked()
}

func IsMachineCrossdev(cn string) bool {
	machinesMutex.RLock()
	defer machinesMutex.RUnlock()

	if machines.Entries[cn].Arch != ServerArch {
		return true
	}

	return false
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
