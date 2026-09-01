package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"git.fedesito.me/fedes1to/eserve/internal/config"
)

// stock stage3 make.conf requests signed binpkgs from the official binhost, which we never use; our builds sign themselves via the lines below
const defaultFlavorMakeConf = `# flavor build config: this wins over the client's make.conf
CFLAGS="-O2 -pipe -march=native"
CXXFLAGS="${CFLAGS}"
FEATURES="${FEATURES} -binpkg-request-signature binpkg-signing"
BINPKG_GPG_SIGNING_BASE_COMMAND="gpg --sign --armor --batch --no-tty --yes [PORTAGE_CONFIG]"
BINPKG_GPG_SIGNING_GPG_HOME="/etc/eserved-gnupg"
BINPKG_GPG_SIGNING_DIGEST="SHA256"
BINPKG_GPG_VERIFY_BASE_COMMAND="gpg --status-fd 2 --verify [PORTAGE_CONFIG] [SIGNATURE]"
BINPKG_GPG_VERIFY_GPG_HOME="/etc/eserved-gnupg"
// portage drops verify to user "nobody" by default, which cant read the root-only keyring
GPG_VERIFY_USER_DROP=""
`

const defaultFlavorBinreposConf = `# no remote binrepos in the build chroot
`

func ListFlavorConfig(flavor string) ([]string, error) {
	entries, err := os.ReadDir(config.FlavorConfigDir(flavor))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name()+"/")
			continue
		}
		names = append(names, entry.Name())
	}
	return names, nil
}

// scaffolds the missing files, never overwrites existing ones
func CreateFlavorConfig(flavor string) (created []string, err error) {
	dir := config.FlavorConfigDir(flavor)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("couldn't create flavor config dir: %w", err)
	}

	files := map[string]string{
		"make.conf":     defaultFlavorMakeConf,
		"binrepos.conf": defaultFlavorBinreposConf,
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			continue // already there, leave it alone
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return created, fmt.Errorf("couldn't write %s: %w", name, err)
		}
		created = append(created, name)
	}
	return created, nil
}
