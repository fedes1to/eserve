package macros

import (
	"os"
	"text/template"
)

type Command struct {
	Name        string
	Description string
}

const usageTmpl = `Usage: {{.Exe}} <command> [options]

Commands:
{{- range .Commands}}
  {{printf "%-12s %s" .Name .Description}}
{{- end}}

Run '{{.Exe}} <command> -help' for command options.
`

func PrintUsage(exe string, commands []Command) {
	tmpl := template.Must(template.New("usage").Parse(usageTmpl))
	tmpl.Execute(os.Stderr, struct {
		Exe      string
		Commands []Command
	}{exe, commands})
}
