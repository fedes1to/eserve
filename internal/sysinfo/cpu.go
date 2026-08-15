package sysinfo

import (
	"fmt"
	"os/exec"
	"strings"
)

func GetCpuChost() (string, error) {
	cmdOutput, cmdError := exec.Command("gcc", "-dumpmachine").Output()
	if cmdError != nil {
		return "", fmt.Errorf("Failed to execute gcc, %w", cmdError)
	}
	return strings.TrimSpace(string(cmdOutput)), nil
}

// returns CPU subarch using gcc
func GetCpuMarch() (string, error) {
	out, _ := exec.Command("gcc", "-march=native", "-Q", "--help=target").Output()
	for line := range strings.SplitSeq(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "-march=" {
			return f[1], nil
		}
	}
	return "", fmt.Errorf("Failed to get CPU subarch")
}
