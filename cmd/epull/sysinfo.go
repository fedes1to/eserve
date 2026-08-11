package main

import (
	"errors"
	"os/exec"
	"strings"
)

// returns CPU subarch using gcc
func getCpuSubarch() (string, error) {
	out, _ := exec.Command("gcc", "-march=native", "-Q", "--help=target").Output()
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "-march=" {
			return f[1], nil
		}
	}
	return "", errors.New("Failed to get CPU subarch")
}
