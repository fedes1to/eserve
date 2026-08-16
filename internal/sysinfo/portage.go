package sysinfo

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func getPortageMarch() (string, error) {
	output, portageError := exec.Command("portageq", "envvar", "CFLAGS").Output()
	if portageError != nil {
		return "", portageError
	}
	flags := string(output)
	for flag := range strings.FieldsSeq(flags) {
		if march, ok := strings.CutPrefix(flag, "-march="); ok {
			return march, nil
		}
	}
	return "", nil // absent
}

func GetPortageProfile() (string, error) {
	makeLink, linkError := os.Readlink("/etc/portage/make.profile")
	if linkError != nil {
		return "", linkError
	}
	_, profile, found := strings.Cut(makeLink, "profiles/")
	if !found {
		return "", fmt.Errorf("unexpected make.profile target: %q", makeLink)
	}
	return profile, nil
}
