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

	lines := strings.Split(string(existing), "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "["+flavor+"]" {
			start = i
			break
		}
	}

	if start != -1 {
		// our keys still current? leave it alone (user-added keys stay)
		syncURI, verifySignature, priority := "", "", ""
		for i := start + 1; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if strings.HasPrefix(trimmed, "[") {
				break
			}
			switch {
			case strings.HasPrefix(trimmed, "sync-uri"):
				syncURI = trimmed
			case strings.HasPrefix(trimmed, "verify-signature"):
				verifySignature = trimmed
			case strings.HasPrefix(trimmed, "priority"):
				priority = trimmed
			}
		}
		if syncURI == "sync-uri = "+binhostURL && verifySignature == "verify-signature = true" && priority == "priority = 10000" {
			return nil
		}

		// replace the section in place (up to the next section or EOF)
		end := len(lines)
		for i := start + 1; i < len(lines); i++ {
			if strings.HasPrefix(strings.TrimSpace(lines[i]), "[") {
				end = i
				break
			}
		}
		for end > start+1 && strings.TrimSpace(lines[end-1]) == "" {
			end--
		}

		replacement := strings.Split(strings.TrimRight(fmt.Sprintf(binhostConfigTemplate, flavor, binhostURL, flavor), "\n"), "\n")
		replacement = append(replacement, "") // blank line before the next section
		newLines := make([]string, 0, len(lines)+len(replacement))
		newLines = append(newLines, lines[:start]...)
		newLines = append(newLines, replacement...)
		newLines = append(newLines, lines[end:]...)
		return os.WriteFile(path, []byte(strings.Join(newLines, "\n")), 0644)
	}

	if err := os.MkdirAll(PortageConfigRoot, 0755); err != nil {
		return err
	}

	section := fmt.Sprintf(binhostConfigTemplate, flavor, binhostURL, flavor)
	data := make([]byte, 0, len(existing)+len(section)+2)
	data = append(data, existing...)
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		data = append(data, '\n')
	}
	data = append(data, '\n')
	data = append(data, section...)

	return os.WriteFile(path, data, 0644)
}
