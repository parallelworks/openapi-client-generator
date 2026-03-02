package templates

import "embed"

//go:embed *.tmpl
var TemplateFS embed.FS
