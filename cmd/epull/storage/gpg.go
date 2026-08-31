package storage

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"git.fedesito.me/fedes1to/eserve/internal/config"
)

// the client's own keyring: the server's public key, marked ultimately trusted
var gpgHome = filepath.Join(config.ClientConfigPath, "gnupg")

// portage only accepts binpkgs signed by keys in this keyring
var verifyMakeConfLines = []string{
	`BINPKG_GPG_VERIFY_BASE_COMMAND="gpg --status-fd 2 --verify [PORTAGE_CONFIG] [SIGNATURE]"`,
	`BINPKG_GPG_VERIFY_GPG_HOME="` + config.ClientConfigPath + `/gnupg"`,
	// portage drops verify to user "nobody" by default, which cant read the root-only keyring
	`GPG_VERIFY_USER_DROP=""`,
}

func SetupGpgKey(armored string) error {
	if err := os.MkdirAll(gpgHome, 0o700); err != nil {
		return fmt.Errorf("couldn't create the gpg home: %w", err)
	}
	importCmd := exec.Command("gpg", "--homedir", gpgHome, "--batch", "--import")
	importCmd.Stdin = strings.NewReader(armored)
	if output, err := importCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gpg import failed: %w: %s", err, output)
	}
	fingerprint, err := fingerprint()
	if err != nil {
		return err
	}
	trustCmd := exec.Command("gpg", "--homedir", gpgHome, "--batch", "--import-ownertrust")
	trustCmd.Stdin = strings.NewReader(fingerprint + ":6:\n")
	if output, err := trustCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gpg ownertrust failed: %w: %s", err, output)
	}
	return upsertMakeConfLines(PortageConfigRoot+"/make.conf", verifyMakeConfLines)
}

func fingerprint() (string, error) {
	output, err := exec.Command("gpg", "--homedir", gpgHome, "--list-keys", "--with-colons").Output()
	if err != nil {
		return "", fmt.Errorf("couldn't list gpg keys: %w", err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, "fpr:") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) >= 10 && fields[9] != "" {
			return fields[9], nil
		}
	}
	return "", fmt.Errorf("no key found after import")
}
func upsertMakeConfLines(path string, lines []string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	existing := []string{}
	if len(data) > 0 {
		existing = strings.Split(string(data), "\n")
	}
	out := make([]string, 0, len(existing)+len(lines))
	for _, line := range existing {
		replaced := false
		for _, want := range lines {
			if strings.HasPrefix(line, want[:strings.Index(want, "=")+1]) {
				out = append(out, want)
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, line)
		}
	}
	for _, want := range lines {
		found := false
		for _, line := range out {
			if line == want {
				found = true
				break
			}
		}
		if !found {
			out = append(out, want)
		}
	}
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
}
