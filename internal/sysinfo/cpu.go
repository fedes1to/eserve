package sysinfo

import (
	"fmt"
	"os/exec"
	"strings"
)

func GetGccMachine() (string, error) {
	cmdOutput, err := exec.Command("gcc", "-dumpmachine").Output()
	if err != nil {
		return "", fmt.Errorf("Failed to execute gcc, %w", err)
	}
	return strings.TrimSpace(string(cmdOutput)), nil
}

func getGccMarch() (string, error) {
	output, err := exec.Command("gcc", "-march=native", "-Q", "--help=target").Output()
	if err != nil {
		return "", fmt.Errorf("Failed to execute gcc, %w", err)
	}
	for line := range strings.SplitSeq(string(output), "\n") {
		field := strings.Fields(line)
		if len(field) >= 2 && field[0] == "-march=" {
			return field[1], nil
		}
	}
	return "", fmt.Errorf("Failed to get CPU subarch")
}

func GetCpuSubarch() (string, error) {
	portageMarch, err := getPortageMarch()
	if err != nil {
		return "", err
	}
	if portageMarch != "" && portageMarch != "native" {
		return portageMarch, nil
	}
	return getGccMarch()
}
