package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/analyzer"
	"github.com/parallelworks/openapi-client-generator/internal/parser"
)

const additionalPropertiesSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /pets/{id}:
    get:
      operationId: getPet
      parameters: [{ name: id, in: path, required: true, schema: { type: string } }]
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Pet" }
components:
  schemas:
    Pet:
      type: object
      required: [name]
      properties:
        name: { type: string }
        tag: { type: string }
      additionalProperties: true
    Labels:
      type: object
      properties:
        owner: { type: string }
      additionalProperties:
        type: string
`

// TestE2E_AdditionalPropertiesRoundTrip generates a client for schemas that mix
// declared properties with additionalProperties, then compiles and RUNS a test
// proving unknown keys survive an unmarshal/marshal round trip instead of being
// dropped or emitted under a literal "-" key.
func TestE2E_AdditionalPropertiesRoundTrip(t *testing.T) {
	specDir := t.TempDir()
	specPath := filepath.Join(specDir, "spec.yaml")
	if err := os.WriteFile(specPath, []byte(additionalPropertiesSpec), 0o644); err != nil {
		t.Fatalf("writing spec: %v", err)
	}

	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	a := analyzer.New(result.Model)
	pkg, err := a.Analyze("petsapi")
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

	tmpDir := t.TempDir()
	goMod := []byte("module additionalprops-e2e-test\n\ngo 1.25.5\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}

	runtimeTest := []byte(`package petsapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUnknownKeysSurviveRoundTrip(t *testing.T) {
	const payload = ` + "`" + `{"name":"rex","nickname":"r","age":3}` + "`" + `

	var p Pet
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Name != "rex" {
		t.Errorf("Name = %q, want rex", p.Name)
	}
	if got := p.AdditionalProperties["nickname"]; got != "r" {
		t.Errorf("AdditionalProperties[nickname] = %v, want r", got)
	}
	if got := p.AdditionalProperties["age"]; got != float64(3) {
		t.Errorf("AdditionalProperties[age] = %v, want 3", got)
	}
	if _, ok := p.AdditionalProperties["name"]; ok {
		t.Error("declared property leaked into AdditionalProperties")
	}

	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), ` + "`" + `"-"` + "`" + `) {
		t.Errorf("marshal emitted a literal \"-\" key: %s", out)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	want := map[string]any{"name": "rex", "nickname": "r", "age": float64(3)}
	if len(got) != len(want) {
		t.Fatalf("round trip = %s, want %v", out, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("round trip[%s] = %v, want %v", k, got[k], v)
		}
	}
}

func TestTypedAdditionalProperties(t *testing.T) {
	var l Labels
	if err := json.Unmarshal([]byte(` + "`" + `{"owner":"me","env":"prod"}` + "`" + `), &l); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if l.Owner == nil || *l.Owner != "me" {
		t.Errorf("Owner = %v, want me", l.Owner)
	}
	if l.AdditionalProperties["env"] != "prod" {
		t.Errorf("AdditionalProperties[env] = %q, want prod", l.AdditionalProperties["env"])
	}

	out, err := json.Marshal(l)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), ` + "`" + `"env":"prod"` + "`" + `) {
		t.Errorf("marshal dropped the additional property: %s", out)
	}
}

func TestEmptyAdditionalPropertiesOmitsNothingExtra(t *testing.T) {
	out, err := json.Marshal(Pet{Name: "rex"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != ` + "`" + `{"name":"rex"}` + "`" + ` {
		t.Errorf("marshal = %s, want {\"name\":\"rex\"}", out)
	}
}
`)
	if err := os.WriteFile(filepath.Join(tmpDir, "additional_properties_test.go"), runtimeTest, 0o644); err != nil {
		t.Fatalf("writing runtime test: %v", err)
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test on generated code failed: %v\n%s", err, string(output))
	}
	t.Logf("additionalProperties round trip test passed:\n%s", string(output))
}
