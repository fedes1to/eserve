package gpg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"git.fedesito.me/fedes1to/eserve/internal/config"
)

// a passphraseless key in a 0700 dir; the build chroot copies the whole dir
var home = filepath.Join(config.ServerConfigPath, "gnupg")

const batchParams = `%no-protection
Key-Type: EDDSA
Key-Curve: Ed25519
Key-Usage: sign
Name-Real: eserved
Name-Email: eserved@localhost
Expire-Date: 0
`

func Home() string {
	return home
}

func EnsureKey() error {
	if _, err := KeyFingerprint(); err == nil {
		return nil
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return fmt.Errorf("couldn't create the gpg home: %w", err)
	}
	paramFile := filepath.Join(home, "batchgen.param")
	if err := os.WriteFile(paramFile, []byte(batchParams), 0o600); err != nil {
		return fmt.Errorf("couldn't write the gpg params: %w", err)
	}
	defer os.Remove(paramFile)
	if output, err := gpgCommand("--batch", "--quiet", "--gen-key", paramFile).CombinedOutput(); err != nil {
		return fmt.Errorf("gpg key generation failed: %w: %s", err, output)
	}
	if _, err := KeyFingerprint(); err != nil {
		return fmt.Errorf("gpg key generation succeeded but no key was found: %w", err)
	}
	return nil
}

func KeyFingerprint() (string, error) {
	output, err := gpgCommand("--list-keys", "--with-colons").Output()
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
	return "", fmt.Errorf("no signing key found")
}

func PublicKey() (string, error) {
	fingerprint, err := KeyFingerprint()
	if err != nil {
		return "", err
	}
	output, err := gpgCommand("--armor", "--export", fingerprint).Output()
	if err != nil {
		return "", fmt.Errorf("couldn't export the public key: %w", err)
	}
	return string(output), nil
}

func CopyTo(dest string) error {
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return err
	}
	return copyDir(home, dest)
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		from := filepath.Join(src, entry.Name())
		to := filepath.Join(dst, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSocket != 0 {
			continue
		}
		if entry.IsDir() {
			if err := os.MkdirAll(to, 0o700); err != nil {
				return err
			}
			if err := copyDir(from, to); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(from)
		if err != nil {
			return err
		}
		if err := os.WriteFile(to, data, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func gpgCommand(args ...string) *exec.Cmd {
	full := append([]string{"--homedir", home}, args...)
	return exec.Command("gpg", full...)
}
