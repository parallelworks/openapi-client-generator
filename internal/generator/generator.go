package generator

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
	"github.com/parallelworks/openapi-client-generator/internal/templates"
)

// Generator executes templates against the IR to produce Go source files.
type Generator struct {
	pkg       *ir.Package
	templates *template.Template
}

// GeneratedFile represents a single generated output file.
type GeneratedFile struct {
	Name    string
	Content []byte
}

// New creates a Generator for the given IR package, parsing all embedded templates.
func New(pkg *ir.Package) (*Generator, error) {
	tmpl, err := template.New("").Funcs(FuncMap()).ParseFS(templates.TemplateFS, "*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	return &Generator{pkg: pkg, templates: tmpl}, nil
}

// Generate executes all templates and returns the generated files.
func (g *Generator) Generate() ([]GeneratedFile, error) {
	specs := []struct {
		tmplName string
		fileName string
	}{
		{"types.go.tmpl", "types.go"},
		{"client.go.tmpl", "client.go"},
		{"options.go.tmpl", "options.go"},
		{"helpers.go.tmpl", "helpers.go"},
		{"operations.go.tmpl", "operations.go"},
		{"pagination.go.tmpl", "pagination.go"},
		{"retry.go.tmpl", "retry.go"},
		{"middleware.go.tmpl", "middleware.go"},
		{"auth.go.tmpl", "auth.go"},
		{"errors.go.tmpl", "errors.go"},
	}

	var files []GeneratedFile
	for _, spec := range specs {
		var buf bytes.Buffer
		if err := g.templates.ExecuteTemplate(&buf, spec.tmplName, g.pkg); err != nil {
			return nil, fmt.Errorf("executing template %s: %w", spec.tmplName, err)
		}
		files = append(files, GeneratedFile{Name: spec.fileName, Content: buf.Bytes()})
	}
	return files, nil
}
