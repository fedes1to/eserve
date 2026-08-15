package sysinfo

import (
	"fmt"
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

func GetCpuChost() (string, error) {
	cmdOutput, cmdError := exec.Command("gcc", "-dumpmachine").Output()
	if cmdError != nil {
		return "", fmt.Errorf("Failed to execute gcc, %w", cmdError)
	}
	return strings.TrimSpace(string(cmdOutput)), nil
}

func getGccMarch() (string, error) {
	output, outputError := exec.Command("gcc", "-march=native", "-Q", "--help=target").Output()
	if outputError != nil {
		return "", fmt.Errorf("Failed to execute gcc, %w", outputError)
	}
	for line := range strings.SplitSeq(string(output), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "-march=" {
			return f[1], nil
		}
	}
	return "", fmt.Errorf("Failed to get CPU subarch")
}

// returns CPU subarch using gcc
func GetCpuMarch() (string, error) {
	portageMarch, portageError := getPortageMarch()
	if portageError != nil {
		return "", portageError
	}
	if portageMarch != "" {
		return portageMarch, nil
	}

	return getGccMarch()
}
