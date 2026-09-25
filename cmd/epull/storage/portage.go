package storage

import (
	"fmt"
	"os"
	"strings"
)

const PortageConfigRoot = "/etc/portage"

const binhostConfigTemplate = `[%s]
priority = 10000
sync-uri = %s
location = /var/cache/binhost/%s
verify-signature = true
`

// a binrepos.conf section: its [name] header line and the lines under it
type binrepoSection struct {
	name  string
	lines []string
}

// the value of a key in the section, "" when it isn't there
func (s binrepoSection) value(key string) string {
	if len(s.lines) < 2 {
		return "" // the nameless preamble, or an empty section
	}
	for _, line := range s.lines[1:] {
		trimmed := strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(trimmed, key)
		if !ok || (rest != "" && rest[0] != '=' && rest[0] != ' ' && rest[0] != '\t') {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rest), "="))
	}
	return ""
}

// our keys still current? then the section is left alone and user-added keys stay
func (s binrepoSection) current(binhostURL string) bool {
	return s.value("sync-uri") == binhostURL &&
		s.value("verify-signature") == "true" &&
		s.value("priority") == "10000"
}

// every section epull writes points at <server>/pkgs/<flavor>, so that prefix is
// what tells ours apart from a user's
func (s binrepoSection) ours(prefix string) bool {
	return strings.HasPrefix(s.value("sync-uri"), prefix)
}

// splits the file into sections, keeping anything before the first header as a
// nameless preamble so comments survive
func splitBinrepoSections(data string) []binrepoSection {
	var sections []binrepoSection
	current := binrepoSection{}
	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			sections = append(sections, current)
			current = binrepoSection{
				name:  strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"),
				lines: []string{line},
			}
			continue
		}
		if len(current.lines) == 0 && trimmed == "" {
			continue // the blank line between two sections
		}
		current.lines = append(current.lines, line)
	}
	return append(sections, current)
}

func renderBinrepoSections(sections []binrepoSection) string {
	var out []string
	for _, section := range sections {
		lines := section.lines
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		if len(lines) == 0 {
			continue
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, lines...)
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

func binhostSectionLines(flavor, binhostURL string) []string {
	return strings.Split(strings.TrimRight(fmt.Sprintf(binhostConfigTemplate, flavor, binhostURL, flavor), "\n"), "\n")
}

func eservedBinhostPrefix(binhostURL string) string {
	if i := strings.LastIndex(binhostURL, "/pkgs/"); i >= 0 {
		return binhostURL[:i+len("/pkgs/")]
	}
	return binhostURL
}

func WriteBinhostConfig(flavor, binhostURL string) error {
	path := PortageConfigRoot + "/binrepos.conf"
	// gentoo ships binrepos.conf as a drop-in directory
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		path = path + "/eserved.conf"
	}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	prefix := eservedBinhostPrefix(binhostURL)
	kept := make([]binrepoSection, 0, len(existing)+1)
	found := false
	for _, section := range splitBinrepoSections(string(existing)) {
		switch {
		case section.name == flavor:
			found = true
			if section.current(binhostURL) {
				kept = append(kept, section) // our keys are current, user-added keys stay
				continue
			}
			kept = append(kept, binrepoSection{name: flavor, lines: binhostSectionLines(flavor, binhostURL)})
		case section.ours(prefix):
			// epull wrote this one for another flavor of the same server; the
			// machine switched away, so it must not keep the old binhost around
		default:
			kept = append(kept, section)
		}
	}
	if !found {
		kept = append(kept, binrepoSection{name: flavor, lines: binhostSectionLines(flavor, binhostURL)})
	}

	rendered := renderBinrepoSections(kept)
	if rendered == string(existing) {
		return nil
	}
	if err := os.MkdirAll(PortageConfigRoot, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(rendered), 0644)
}
