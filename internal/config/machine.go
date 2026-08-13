package config

import (
	"fmt"
	"sync"
)

type MachineEntry struct {
	March       string `json:"march"`
	Flavor      string `json:"flavor"`
	Threads     int    `json:"threads"` // 0 if not specified (unlimited)
	Fingerprint string `json:"fingerprint"`
}

// we still address by tokens here, thats the GUID we will use internally
type MachinesFile struct {
	Entries map[string]MachineEntry `json:"machines"`
}

var (
	machines      MachinesFile
	machinesMutex sync.RWMutex
	machinesPath  string = ServerConfigPath + "machines.json"
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
	if saveError := SafeSaveJsonFile(machinesPath, machines); saveError != nil {
		return saveError
	}
	return nil
}

// READ THE FUCKING NAME, USE ONLY WHEN LOCKED
func loadMachinesLocked() error {
	if loadError := LoadJsonFile(machinesPath, &machines); loadError != nil {
		return loadError
	}

	if machines.Entries == nil {
		machines.Entries = make(map[string]MachineEntry)
	}

	return nil
}

// there should be a better way to do this, go please add func overloading please
func CreateMachine(hostname string, march string, flavor string, fingerprint string, threads ...int) error {
	var actualThreads int // go is just stupid like that
	if len(threads) > 0 {
		actualThreads = threads[0]
	} else {
		actualThreads = 0
	}

	machinesMutex.Lock()
	defer machinesMutex.Unlock()

	entry, entryExists := machines.Entries[hostname]
	if entryExists {
		return fmt.Errorf("Tried to add an existing machine")
	}

	entry.March = march
	entry.Flavor = flavor
	entry.Threads = actualThreads
	machines.Entries[hostname] = entry
	return saveMachinesLocked()
}
