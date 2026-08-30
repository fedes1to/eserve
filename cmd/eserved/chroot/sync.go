package chroot

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/flavorlock"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const ConfigDir = ".eserved"

const maxExtractedBytes = 256 << 20

var syncedPathSet = func() map[string]bool {
	set := make(map[string]bool, len(protocol.PortageSyncPaths))
	for _, p := range protocol.PortageSyncPaths {
		set[p] = true
	}
	return set
}()

func validFlavor(flavor string) bool {
	if flavor == "" || flavor == "." || flavor == ".." {
		return false
	}
	return !strings.ContainsAny(flavor, "/\\ ")
}

func chrootDir(flavor string) string {
	return filepath.Join(serverConfig.Settings.ChrootBase, flavor)
}

func SyncArchivePath(flavor, cn string) string {
	return filepath.Join(serverConfig.ServerConfigPath, "sync", flavor, cn+".tar.gz")
}

func IsProvisioned(flavor string) bool {
	if !validFlavor(flavor) {
		return false
	}
	_, err := os.Stat(filepath.Join(chrootDir(flavor), "etc", stageMarker))
	return err == nil
}

func ApplySync(ctx context.Context, flavor, claimedFingerprint, archivePath string) (string, error) {
	if !validFlavor(flavor) {
		return "", fmt.Errorf("invalid flavor %q", flavor)
	}
	unlock := flavorlock.Lock(flavor)
	defer unlock()

	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	root, err := os.OpenRoot(chrootDir(flavor))
	if err != nil {
		return "", fmt.Errorf("couldn't open chroot: %w", err)
	}
	defer root.Close()

	sweepStaleConfigDirs(root, chrootDir(flavor))

	suffix, err := randomSuffix()
	if err != nil {
		return "", err
	}
	staging := ConfigDir + ".new-" + suffix

	present, fingerprint, err := buildStagedConfig(ctx, root, staging, flavor, archivePath)
	if err != nil {
		root.RemoveAll(staging)
		return "", err
	}
	if fingerprint != claimedFingerprint {
		root.RemoveAll(staging)
		return "", fmt.Errorf("archive fingerprint %s doesn't match the claimed %s", fingerprint, claimedFingerprint)
	}

	if err := installStagedConfig(root, staging, present); err != nil {
		return "", err
	}
	return fingerprint, nil
}

func extractArchive(ctx context.Context, archivePath string, root *os.Root, staging string) (present map[string]bool, fingerprint string, err error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, "", fmt.Errorf("couldn't open sync archive: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, "", fmt.Errorf("sync archive is not gzip: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	manifest := protocol.NewPortageManifest()
	present = make(map[string]bool)
	var extracted int64

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return present, manifest.Fingerprint(), nil
		}
		if err != nil {
			return nil, "", fmt.Errorf("malformed sync archive: %w", err)
		}
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}

		name, err := validateMemberName(header.Name)
		if err != nil {
			return nil, "", err
		}
		if name == "" {
			continue
		}

		top, _, _ := strings.Cut(name, "/")
		if !syncedPathSet[top] {
			continue
		}
		present[top] = true

		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(staging+"/"+name, 0o755); err != nil {
				return nil, "", fmt.Errorf("couldn't create directory %q: %w", name, err)
			}
			manifest.AddDirectory(name)
		case tar.TypeReg, tar.TypeRegA:
			written, digest, err := writeMemberFile(root, staging+"/"+name, tarReader, &extracted)
			if err != nil {
				return nil, "", err
			}
			manifest.AddFile(name, written, digest)
		default:
			return nil, "", fmt.Errorf("unsupported entry %q in sync archive, only regular files and directories are synced", header.Name)
		}
	}
}

func validateMemberName(name string) (string, error) {
	if name == "" {
		return "", nil
	}
	if strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("absolute path %q in sync archive", name)
	}
	for _, component := range strings.Split(name, "/") {
		if component == ".." {
			return "", fmt.Errorf("path traversal %q in sync archive", name)
		}
	}
	return path.Clean(name), nil
}

func writeMemberFile(root *os.Root, name string, tarReader *tar.Reader, total *int64) (int64, []byte, error) {
	if err := root.MkdirAll(path.Dir(name), 0o755); err != nil {
		return 0, nil, fmt.Errorf("couldn't create directory for %q: %w", name, err)
	}

	out, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, nil, fmt.Errorf("couldn't write %q: %w", name, err)
	}

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(out, hash), tarReader)
	if err != nil {
		out.Close()
		return 0, nil, fmt.Errorf("couldn't write %q: %w", name, err)
	}
	*total += written
	if *total > maxExtractedBytes {
		out.Close()
		return 0, nil, fmt.Errorf("sync archive expands beyond %d bytes", maxExtractedBytes)
	}
	if err := out.Close(); err != nil {
		return 0, nil, fmt.Errorf("couldn't finish writing %q: %w", name, err)
	}
	return written, hash.Sum(nil), nil
}

func linkSyncedPaths(root *os.Root, present map[string]bool) error {
	if err := root.MkdirAll("etc/portage", 0o755); err != nil {
		return fmt.Errorf("couldn't open etc/portage: %w", err)
	}

	for _, p := range protocol.PortageSyncPaths {
		dest := "etc/portage/" + p
		target := "../../" + ConfigDir + "/" + p

		if !present[p] {
			// only drop links that point where we link, leave the rest alone
			if info, err := root.Lstat(dest); err == nil && info.Mode()&os.ModeSymlink != 0 {
				if current, err := root.Readlink(dest); err == nil && current == target {
					if err := root.Remove(dest); err != nil {
						return fmt.Errorf("couldn't remove stale link %q: %w", p, err)
					}
				}
			}
			continue
		}

		if info, err := root.Lstat(dest); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				if current, err := root.Readlink(dest); err == nil && current == target {
					continue
				}
			}
			if err := removePath(root, dest); err != nil {
				return fmt.Errorf("couldn't replace %q: %w", p, err)
			}
		}
		if err := root.Symlink(target, dest); err != nil {
			return fmt.Errorf("couldn't link %q: %w", p, err)
		}
	}
	return nil
}

// leftover staging/trash dirs from a sync that crashed mid-flight
func sweepStaleConfigDirs(root *os.Root, chrootDir string) {
	entries, err := os.ReadDir(chrootDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ConfigDir+".new-") || strings.HasPrefix(name, ConfigDir+".old-") {
			root.RemoveAll(name)
		}
	}
}

func removePath(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		return root.RemoveAll(name)
	}
	return root.Remove(name)
}

func randomSuffix() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
