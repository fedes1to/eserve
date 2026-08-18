package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"text/template"
)

const binhostConfigTemplate = `[{{.Flavor}}]
priority = 9999
sync-type = websync
sync-uri = {{.BinhostURL}}
location = /var/cache/binhost/{{.Flavor}}
verify-signature = true
`

type binhostConfigData struct {
	Flavor     string
	BinhostURL string
}

func WriteBinhostConfig(flavor, binhostURL string) error {
	var config bytes.Buffer
	tmpl, err := template.New("binhost").Parse(binhostConfigTemplate)
	if err != nil {
		return err
	}

	err = tmpl.Execute(&config, binhostConfigData{
		Flavor:     flavor,
		BinhostURL: binhostURL,
	})
	if err != nil {
		return err
	}

	dir := "/etc/portage/binrepos.conf"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, flavor+".conf")
	return os.WriteFile(path, config.Bytes(), 0644)
}
