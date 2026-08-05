package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/analyzer"
	"github.com/parallelworks/openapi-client-generator/internal/parser"
	"github.com/parallelworks/openapi-client-generator/internal/templates"
)

// generateAndBuild generates a client for spec, builds it, and returns the
// compiler output ("" when it built) alongside types.go.
func generateAndBuild(t *testing.T, spec string) (buildOutput, types string) {
	t.Helper()

	specPath := filepath.Join(t.TempDir(), "spec.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("writing spec: %v", err)
	}
	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	pkg, err := analyzer.New(result.Model).Analyze("probe")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New generator: %v", err)
	}
	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module probe\n\ngo 1.25.5\n"), 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := WriteFiles(dir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		out = nil
	}
	for _, f := range files {
		if f.Name == "types.go" {
			types = string(f.Content)
		}
	}
	return string(out), types
}

// TestE2E_SchemaNamedLikeGeneratedType checks that a schema whose name matches
// one of the identifiers the templates always declare is renamed rather than
// redeclared. Go has one package scope, so the collision would not compile.
func TestE2E_SchemaNamedLikeGeneratedType(t *testing.T) {
	for _, name := range templates.ReservedIdentifiers {
		t.Run(name, func(t *testing.T) {
			build, types := generateAndBuild(t, `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /u:
    post:
      operationId: upload
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema: { $ref: "#/components/schemas/Upload" }
      responses: { "204": { description: ok } }
components:
  schemas:
    Upload:
      type: object
      required: [file]
      properties:
        file: { type: string, format: binary }
    `+name+`:
      type: object
      properties:
        x: { type: string }
`)
			if build != "" {
				t.Errorf("a schema named %q does not compile:\n%s", name, build)
			}
			if !strings.Contains(types, "type "+name+"2 struct") {
				t.Errorf("schema %q was not renamed out of the way:\n%s", name, types)
			}
		})
	}
}

// TestE2E_ForwardReferenceUsesTheRenamedType checks that a reference to a schema
// converted later still resolves to the name that schema ends up with. Two
// schemas that differ only in punctuation share one exported spelling, so the
// second is renamed — and a forward reference used to silently point at the first.
func TestE2E_ForwardReferenceUsesTheRenamedType(t *testing.T) {
	build, types := generateAndBuild(t, `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Holder:
      type: object
      properties:
        a: { $ref: "#/components/schemas/foo-bar" }
        b: { $ref: "#/components/schemas/foo_bar" }
    foo-bar:
      type: object
      properties: { p: { type: string } }
    foo_bar:
      type: object
      properties: { q: { type: integer } }
`)
	if build != "" {
		t.Fatalf("generated client does not compile:\n%s", build)
	}
	if !strings.Contains(types, "A *FooBar `") {
		t.Errorf("Holder.a does not reference FooBar:\n%s", types)
	}
	if !strings.Contains(types, "B *FooBar2 `") {
		t.Errorf("Holder.b does not reference the renamed FooBar2:\n%s", types)
	}
}
