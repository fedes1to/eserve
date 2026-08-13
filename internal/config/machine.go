package config

import (
	"fmt"
	"sync"
)

type MachineEntry struct {
	March   string `json:"march"`
	Flavor  string `json:"flavor"`
	Threads int    `json:"threads"` // 0 if not specified (unlimited)
}

// we still address by tokens here, thats the GUID we will use internally
type MachinesFile struct {
	Entries map[string]MachineEntry `json:"machines"`
}

var (
	machines      MachinesFile
	machinesMutex sync.Mutex
)

// there should be a better way to do this, go please add func overloading please
func CreateMachine(token string, march string, flavor string, threads ...int) error {
	var actualThreads int // go is just stupid like that
	if len(threads) > 0 {
		actualThreads = threads[0]
	} else {
		actualThreads = 0
	}

	machinesMutex.Lock()
	defer machinesMutex.Unlock()

	entry, entryExists := machines.Entries[token]
	if entryExists {
		return fmt.Errorf("Tried to add an existing machine")
	}

	entry.March = march
	entry.Flavor = flavor
	entry.Threads = actualThreads
	machines.Entries[token] = entry
	return nil
}

func RefreshMachines() error {

	return nil
}
