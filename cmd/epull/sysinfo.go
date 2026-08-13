package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// returns CPU subarch using gcc
func getCpuSubarch() (string, error) {
	out, _ := exec.Command("gcc", "-march=native", "-Q", "--help=target").Output()
	for line := range strings.SplitSeq(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "-march=" {
			return f[1], nil
		}
	}
	return "", fmt.Errorf("Failed to get CPU subarch")
}
