package storage

import (
	"fmt"
	"os"
	"strings"
)

const PortageConfigRoot = "/etc/portage"

// the binhost is unsigned, verify-signature stays off until signing lands
const binhostConfigTemplate = `[%s]
priority = 9999
sync-uri = %s
location = /var/cache/binhost/%s
verify-signature = false
`

func WriteBinhostConfig(flavor, binhostURL string) error {
	path := PortageConfigRoot + "/binrepos.conf"
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
		// the section's old sync-uri is still current? leave it alone
		for i := start + 1; i < len(lines); i++ {
			if strings.HasPrefix(strings.TrimSpace(lines[i]), "sync-uri") {
				if strings.TrimSpace(lines[i]) == "sync-uri = "+binhostURL {
					return nil
				}
				break
			}
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
