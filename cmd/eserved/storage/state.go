package storage

import (
	"path/filepath"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/config"
)

// state files written before SafeSaveJsonFile got its 0600 keep their old mode
// until something rewrites them, so tighten what eserved owns at startup. nothing
// non-root reads these: builds run as root inside bwrap/chroot, and the CA portage
// trusts is the copy in /etc/ssl/certs
func TightenStatePermissions() error {
	root := serverConfig.ServerConfigPath
	for _, name := range []string{"settings.json", "tokens.json", "machines.json", "ca.key", "server.key"} {
		if err := config.TightenMode(filepath.Join(root, name), 0o600); err != nil {
			return err
		}
	}
	for _, name := range []string{"sync", "jobs"} {
		if err := config.TightenTree(filepath.Join(root, name), 0o700, 0o600); err != nil {
			return err
		}
	}
	return nil
}
