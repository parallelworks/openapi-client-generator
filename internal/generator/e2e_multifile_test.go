package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/analyzer"
	"github.com/parallelworks/openapi-client-generator/internal/parser"
)

// TestE2E_MultiFileSpec covers a spec split across files, which is how large
// specs are written. Relative references resolved against the working directory,
// so generation failed outright unless it ran from the spec's own directory.
func TestE2E_MultiFileSpec(t *testing.T) {
	specDir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(specDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	write("common.yaml", `components:
  schemas:
    Shared:
      type: object
      properties:
        id: { type: string }
        label: { type: string }
      required: [id]
`)
	write("api.yaml", `openapi: 3.1.0
info: { title: multi, version: "1" }
paths:
  /things:
    get:
      operationId: listThings
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: array
                items: { $ref: "./common.yaml#/components/schemas/Shared" }
components:
  schemas:
    Wrapper:
      type: object
      properties:
        primary: { $ref: "./common.yaml#/components/schemas/Shared" }
        secondary: { $ref: "./common.yaml#/components/schemas/Shared" }
`)

	result, err := parser.Parse(filepath.Join(specDir, "api.yaml"), parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	pkg, err := analyzer.New(result.Model).Analyze("multiapi")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	runGeneratedWireTest(t, files, "multifile", `package multiapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A schema in another file is named for itself, not for whichever property in
// this document reached it first, and every reference to it shares that type.
func TestExternalSchemaIsOneNamedType(t *testing.T) {
	var w Wrapper
	if err := json.Unmarshal([]byte(`+"`"+`{"primary":{"id":"a"},"secondary":{"id":"b","label":"l"}}`+"`"+`), &w); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if w.Primary == nil || w.Primary.ID != "a" {
		t.Errorf("primary = %+v", w.Primary)
	}
	if w.Secondary == nil || w.Secondary.Label == nil || *w.Secondary.Label != "l" {
		t.Errorf("secondary = %+v", w.Secondary)
	}

	// Both properties hold the same type, so a value moves between them.
	w.Primary = w.Secondary
	if w.Primary.ID != "b" {
		t.Errorf("primary = %+v", w.Primary)
	}
}

func TestOperationDecodesTheExternalSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`+"`"+`[{"id":"x","label":"y"}]`+"`"+`))
	}))
	defer srv.Close()

	things, err := NewClient(srv.URL).ListThings(t.Context())
	if err != nil {
		t.Fatalf("ListThings: %v", err)
	}
	if things == nil || len(*things) != 1 || (*things)[0].ID != "x" {
		t.Errorf("things = %+v", things)
	}
}
`)
}
