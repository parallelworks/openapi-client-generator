package parser

import (
	"os"
	"path/filepath"
	"testing"
)

// A $ref is relative to the document holding it. Resolving it against the
// working directory instead means a multi-file spec builds only when the
// generator happens to run from the spec's own directory.
func TestParse_RelativeFileRefsResolveAgainstTheSpec(t *testing.T) {
	dir := t.TempDir()
	writeSpecFile(t, dir, "common.yaml", `components:
  schemas:
    Shared:
      type: object
      properties:
        id: { type: string }
      required: [id]
`)
	writeSpecFile(t, dir, "api.yaml", `openapi: 3.1.0
info: { title: multi, version: "1" }
paths: {}
components:
  schemas:
    Local:
      type: object
      properties:
        shared: { $ref: "./common.yaml#/components/schemas/Shared" }
`)

	// Deliberately not the spec's directory: this is the CI case.
	chdir(t, t.TempDir())

	result, err := Parse(filepath.Join(dir, "api.yaml"), Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	schemas := result.Model.Components.Schemas
	local, ok := schemas.Get("Local")
	if !ok {
		t.Fatal("Local not found")
	}
	built, err := local.Schema().Properties.GetOrZero("shared").BuildSchema()
	if err != nil {
		t.Fatalf("building the referenced schema: %v", err)
	}
	if built == nil || built.Properties == nil || built.Properties.Len() != 1 {
		t.Fatalf("the reference into common.yaml did not resolve: %+v", built)
	}
	if _, ok := built.Properties.Get("id"); !ok {
		t.Error("the resolved schema is missing the property it declares")
	}
}

// A spec may reference a file outside its own directory, so the base path is a
// starting point rather than a boundary.
func TestParse_RefsAboveTheSpecDirectoryStillResolve(t *testing.T) {
	root := t.TempDir()
	writeSpecFile(t, root, "common.yaml", `components:
  schemas:
    Shared:
      type: object
      properties:
        id: { type: string }
`)
	specDir := filepath.Join(root, "api")
	if err := os.Mkdir(specDir, 0o755); err != nil {
		t.Fatalf("creating the spec directory: %v", err)
	}
	writeSpecFile(t, specDir, "api.yaml", `openapi: 3.1.0
info: { title: multi, version: "1" }
paths: {}
components:
  schemas:
    Local:
      type: object
      properties:
        shared: { $ref: "../common.yaml#/components/schemas/Shared" }
`)

	chdir(t, t.TempDir())

	if _, err := Parse(filepath.Join(specDir, "api.yaml"), Config{}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
}

func writeSpecFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(previous) })
}
