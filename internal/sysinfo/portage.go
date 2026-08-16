package sysinfo

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func getPortageMarch() (string, error) {
	output, err := exec.Command("portageq", "envvar", "CFLAGS").Output()
	if err != nil {
		return "", err
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
	makeLink, err := os.Readlink("/etc/portage/make.profile")
	if err != nil {
		return "", err
	}
	_, profile, found := strings.Cut(makeLink, "profiles/")
	if !found {
		return "", fmt.Errorf("unexpected make.profile target: %q", makeLink)
	}
	return profile, nil
}
