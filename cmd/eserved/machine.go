package main

import ()

type MachineEntry struct {
	Token   string `json:"token"`
	March   string `json:"march"`
	Flavor  string `json:"flavor"`
	Threads int    `json:"threads"`
}

type MachinesFile struct {
	Machines map[string]MachineEntry `json:"machines"`
}
