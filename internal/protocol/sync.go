package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

// what we sync from the client into the flavor's chroot, both sides
// of the wire share the same list
var PortageSyncPaths = []string{
	"make.conf",
	"env",
	"bashrc",
	"categories",
	"color.map",
	"license_groups",
	"mirrors",
	"modules",
	"package.accept_keywords",
	"package.accept_keywords.d",
	"package.accept_restrict",
	"package.env",
	"package.env.d",
	"package.keywords",
	"package.license",
	"package.license.d",
	"package.mask",
	"package.mask.d",
	"package.properties",
	"package.properties.d",
	"package.unmask",
	"package.unmask.d",
	"package.use",
	"package.use.d",
	"sets",
	"sets.conf",
	"patches",
	"repos.conf",
	"repos.conf.d",
	"binrepos.conf",
	"binrepos.d",
	"profile",
	"postsync.d",
	"repo.postsync.d",
}

type SyncCheckResponse struct {
	Flavor      string `json:"flavor"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// one line per path, in the same order as the archive; epull builds it
// while packing, eserved while extracting, both must digest the same
type PortageManifest struct {
	lines []string
}

func NewPortageManifest() *PortageManifest {
	return &PortageManifest{}
}

func (m *PortageManifest) AddDirectory(path string) {
	m.lines = append(m.lines, fmt.Sprintf("%s\tdirectory\n", filepath.ToSlash(path)))
}

func (m *PortageManifest) AddFile(path string, size int64, digest []byte) {
	m.lines = append(m.lines, fmt.Sprintf("%s\tfile\t%d\t%s\n", filepath.ToSlash(path), size, hex.EncodeToString(digest)))
}

func (m *PortageManifest) Fingerprint() string {
	sum := sha256.Sum256([]byte(strings.Join(m.lines, "")))
	return hex.EncodeToString(sum[:])
}
