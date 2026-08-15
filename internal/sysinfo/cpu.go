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

func GetGccMachine() (string, string, error) {
	cmdOutput, cmdError := exec.Command("gcc", "-dumpmachine").Output()
	if cmdError != nil {
		return "", "", fmt.Errorf("Failed to execute gcc, %w", cmdError)
	}

	parts := strings.Split(strings.TrimSpace(string(cmdOutput)), "-")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("unexpected gcc -dumpmachine output: %q", string(cmdOutput))
	}

	arch := parts[0]
	libc := parts[len(parts)-1]
	return arch, libc, nil
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

func GetCpuSubarch() (string, error) {
	portageMarch, portageError := getPortageMarch()
	if portageError != nil {
		return "", portageError
	}
	if portageMarch != "" {
		return portageMarch, nil
	}

	return getGccMarch()
}
