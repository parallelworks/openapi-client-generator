package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/analyzer"
	"github.com/parallelworks/openapi-client-generator/internal/parser"
)

// TestE2E_InlineUnionRuntimeDispatch generates a client from a spec whose map
// values are an inline discriminated oneOf, then compiles and RUNS a test
// against the generated code to prove UnmarshalJSON dispatches the
// discriminator to the right variant type at runtime.
func TestE2E_InlineUnionRuntimeDispatch(t *testing.T) {
	specPath := filepath.Join(projectRoot(), "testdata", "complex-schemas.yaml")

	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	a := analyzer.New(result.Model)
	pkg, err := a.Analyze("complexschemas")
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
	goMod := []byte("module complex-e2e-test\n\ngo 1.25.5\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}

	runtimeTest := []byte(`package complexschemas

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUnionValueTypeSwitch(t *testing.T) {
	payload := []byte(` + "`" + `{
		"shapes": {
			"a": {"shapeType": "circle", "radius": 2.5},
			"b": {"shapeType": "rectangle", "width": 3, "height": 4}
		}
	}` + "`" + `)

	var sc ShapeCollection
	if err := json.Unmarshal(payload, &sc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	switch v := sc.Shapes["a"].Value.(type) {
	case Circle:
		if v.Radius != 2.5 {
			t.Errorf("circle radius = %v, want 2.5", v.Radius)
		}
	default:
		t.Fatalf("shapes[a].Value = %T, want Circle", sc.Shapes["a"].Value)
	}

	switch v := sc.Shapes["b"].Value.(type) {
	case Rectangle:
		if v.Width != 3 || v.Height != 4 {
			t.Errorf("rectangle = %+v, want width 3 height 4", v)
		}
	default:
		t.Fatalf("shapes[b].Value = %T, want Rectangle", sc.Shapes["b"].Value)
	}

	out, err := json.Marshal(sc.Shapes["a"])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), ` + "`" + `"shapeType":"circle"` + "`" + `) {
		t.Errorf("marshal lost the discriminator: %s", out)
	}

	var bad ShapeCollectionShapesValue
	if err := json.Unmarshal([]byte(` + "`" + `{"shapeType":"hexagon"}` + "`" + `), &bad); err == nil {
		t.Fatal("expected an error for an unknown shapeType, got nil")
	}
}
`)
	if err := os.WriteFile(filepath.Join(tmpDir, "union_runtime_test.go"), runtimeTest, 0o644); err != nil {
		t.Fatalf("writing runtime test: %v", err)
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test on generated code failed: %v\n%s", err, string(output))
	}
	t.Logf("runtime dispatch test passed:\n%s", string(output))
}
