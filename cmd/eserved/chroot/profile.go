package chroot

import (
	"strings"

	"git.fedesito.me/fedes1to/eserve/internal/sysinfo"
)

var serverGccMachine string

type Profile struct {
	Full       string
	GccMachine string
}

func (profile Profile) IsCrossdev() bool {
	clientArch, _, _ := strings.Cut(profile.GccMachine, "-")
	serverArch, _, _ := strings.Cut(serverGccMachine, "-")
	return clientArch != serverArch
}

func InitializeGccInfo() error {
	gccMachine, getError := sysinfo.GetGccMachine()
	if getError != nil {
		return getError
	}
	serverGccMachine = gccMachine
	return nil
}
