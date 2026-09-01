package chroot

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/jobs"
	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
)

// each build copies the binpkgs into an immutable snapshot, the live index points at it

const (
	maxSnapshots  = 3
	maxPublished  = 64 << 20 // per-file cap, gpkg.tars are small
	maxIndexBytes = 8 << 20  // a Packages index should never be megabytes
)

func binhostDir(flavor string) string {
	return filepath.Join(serverConfig.Settings.RepoBase, flavor)
}

func isPublishable(rel string, size int64) bool {
	if size <= 0 || size > maxPublished {
		return false // 0-byte leftovers and partial downloads, no thanks
	}
	return strings.HasSuffix(rel, ".gpkg.tar") || strings.HasSuffix(rel, ".xpak")
}

func PublishBinpkgs(job *jobs.Job, flavor string) (err error) {
	if !ValidFlavor(flavor) {
		return fmt.Errorf("invalid flavor %q", flavor)
	}
	pkgDir := filepath.Join(chrootDir(flavor), "var/cache/binpkgs")
	if _, err := os.Stat(pkgDir); err != nil {
		return fmt.Errorf("no binpkg dir in the chroot, did the build produce any: %w", err)
	}

	type publishFile struct {
		rel  string
		src  string
		size int64
	}
	var files []publishFile
	indexData, hasIndex := readIndexFile(filepath.Join(pkgDir, "Packages"))

	var walkErr error
	filepath.Walk(pkgDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(pkgDir, path)
		if err != nil {
			return err
		}
		if !isPublishable(rel, info.Size()) {
			return nil
		}
		files = append(files, publishFile{rel: rel, src: path, size: info.Size()})
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("couldn't read the binpkg dir: %w", walkErr)
	}

	if !hasIndex {
		return fmt.Errorf("the binpkg dir has no Packages index, nothing to publish")
	}
	hasPackages := false
	for _, f := range files {
		if strings.HasSuffix(f.rel, ".gpkg.tar") || strings.HasSuffix(f.rel, ".xpak") {
			hasPackages = true
			break
		}
	}
	if !hasPackages {
		return fmt.Errorf("the build didn't produce any binpkgs")
	}

	now := time.Now().Unix()
	// two publishes in the same second must not share a dir
	for fileExists(filepath.Join(binhostDir(flavor), fmt.Sprintf("snapshot-%d", now))) {
		now++
	}
	snapshot := filepath.Join(binhostDir(flavor), fmt.Sprintf("snapshot-%d", now))
	baseURL := strings.TrimRight(serverConfig.Settings.BaseBinhostURL, "/") + "/" + flavor + fmt.Sprintf("/snapshot-%d/", now)

	if err := os.MkdirAll(snapshot, 0o755); err != nil {
		return fmt.Errorf("couldn't create snapshot dir: %w", err)
	}
	defer func() {
		// a failed publish leaves no half-built snapshot
		if err != nil {
			os.RemoveAll(snapshot)
		}
	}()

	for _, f := range files {
		dest := filepath.Join(snapshot, f.rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("couldn't create dir for %s: %w", f.rel, err)
		}
		if err := copyFile(f.src, dest); err != nil {
			return fmt.Errorf("couldn't copy %s: %w", f.rel, err)
		}
	}

	newIndex := rewriteIndexURI(indexData, baseURL)

	// the chroot's Packages.gz is stale, both indexes get rewritten
	if err := os.WriteFile(filepath.Join(snapshot, "Packages"), []byte(newIndex), 0o644); err != nil {
		return fmt.Errorf("couldn't write the snapshot index: %w", err)
	}
	if err := writeGzipped(filepath.Join(snapshot, "Packages.gz"), newIndex); err != nil {
		return fmt.Errorf("couldn't write the snapshot index (gz): %w", err)
	}

	if err := os.MkdirAll(binhostDir(flavor), 0o755); err != nil {
		return fmt.Errorf("couldn't create binhost dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(binhostDir(flavor), "Packages"), []byte(newIndex), 0o644); err != nil {
		return fmt.Errorf("couldn't write the binhost index: %w", err)
	}
	if err := writeGzipped(filepath.Join(binhostDir(flavor), "Packages.gz"), newIndex); err != nil {
		return fmt.Errorf("couldn't write the compressed binhost index: %w", err)
	}

	pruneSnapshots(binhostDir(flavor))
	job.WriteProgress(fmt.Sprintf("published %d packages to %s", len(files), baseURL))
	return nil
}

func readIndexFile(path string) (data string, ok bool) {
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 || info.Size() > maxIndexBytes {
		return "", false
	}
	dataBytes, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(dataBytes), true
}

// the client joins this URI with each package's PATH to fetch
func rewriteIndexURI(data, uri string) string {
	lines := strings.Split(data, "\n")
	headerEnd := len(lines) // the header is everything before the first empty line
	for i, line := range lines {
		if line == "" {
			headerEnd = i
			break
		}
	}

	var rebuilt []string
	for _, line := range lines[:headerEnd] {
		if strings.HasPrefix(line, "URI:") {
			continue // drop any stale URI line, we're adding a fresh one
		}
		rebuilt = append(rebuilt, line)
	}
	rebuilt = append(rebuilt, "URI: "+uri)
	return strings.Join(append(append(rebuilt, lines[headerEnd:]...), ""), "\n")
}

func writeGzipped(path, data string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	gz := gzip.NewWriter(file)
	if _, err := io.WriteString(gz, data); err != nil {
		return err
	}
	return gz.Close()
}

func pruneSnapshots(binhostDir string) {
	entries, err := os.ReadDir(binhostDir)
	if err != nil {
		return
	}
	var snapshots []string
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "snapshot-") {
			continue
		}
		snapshots = append(snapshots, entry.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(snapshots)))
	if len(snapshots) > maxSnapshots {
		for _, name := range snapshots[maxSnapshots:] {
			os.RemoveAll(filepath.Join(binhostDir, name))
		}
	}
}

func copyFile(src, dest string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, source); err != nil {
		return err
	}
	return destFile.Close()
}
