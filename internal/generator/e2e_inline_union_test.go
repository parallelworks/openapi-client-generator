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

	var unknown ShapeCollectionShapesValue
	if err := json.Unmarshal([]byte(` + "`" + `{"shapeType":"hexagon","sides":6}` + "`" + `), &unknown); err != nil {
		t.Fatalf("unknown shapeType should decode, got: %v", err)
	}
	if !unknown.IsUnknownVariant() {
		t.Error("expected IsUnknownVariant for an unmapped shapeType")
	}
	if unknown.Value != nil {
		t.Errorf("Value = %v, want nil for an unknown variant", unknown.Value)
	}
	if unknown.UnknownDiscriminator() != "hexagon" {
		t.Errorf("UnknownDiscriminator() = %q, want hexagon", unknown.UnknownDiscriminator())
	}

	back, err := json.Marshal(unknown)
	if err != nil {
		t.Fatalf("marshal unknown: %v", err)
	}
	if !strings.Contains(string(back), ` + "`" + `"sides":6` + "`" + `) {
		t.Errorf("re-marshal lost the unknown variant's payload: %s", back)
	}
}

func TestUnknownVariantDoesNotFailSiblings(t *testing.T) {
	payload := []byte(` + "`" + `{
		"shapes": {
			"a": {"shapeType": "circle", "radius": 2.5},
			"b": {"shapeType": "triangle", "base": 2, "height": 3}
		}
	}` + "`" + `)

	var sc ShapeCollection
	if err := json.Unmarshal(payload, &sc); err != nil {
		t.Fatalf("one unknown variant failed the whole decode: %v", err)
	}
	if _, ok := sc.Shapes["a"].Value.(Circle); !ok {
		t.Fatalf("shapes[a].Value = %T, want Circle", sc.Shapes["a"].Value)
	}
	if !sc.Shapes["b"].IsUnknownVariant() {
		t.Error("shapes[b] should be an unknown variant")
	}
}

// A payload with no discriminator at all identifies nothing, so it must stay an
// error rather than masquerading as an unknown variant.
func TestMissingDiscriminatorIsAnError(t *testing.T) {
	var v ShapeCollectionShapesValue
	if err := json.Unmarshal([]byte(` + "`" + `{"radius":2.5}` + "`" + `), &v); err == nil {
		t.Fatal("expected an error for a payload with no shapeType, got nil")
	}
}

// A union must stay comparable: it is an ordinary field of the structs that hold
// it, so storing the preserved raw payload in a slice would make every one of
// those structs uncomparable too -- a compile error for consumers.
func TestUnionIsComparable(t *testing.T) {
	var a, b ShapeCollectionShapesValue
	if a != b {
		t.Error("zero unions should be equal")
	}
	if !map[ShapeCollectionShapesValue]bool{a: true}[b] {
		t.Error("a union should be usable as a map key")
	}
}

func TestUnknownVariantRawIsACopy(t *testing.T) {
	payload := []byte("{\"shapeType\":\"hexagon\",\"sides\":6}")
	var v ShapeCollectionShapesValue
	if err := json.Unmarshal(payload, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	raw := v.Raw()
	if len(raw) == 0 {
		t.Fatal("Raw() lost the unrecognized payload")
	}
	raw[0] = 'X'
	if again := v.Raw(); again[0] == 'X' {
		t.Error("Raw() aliases the union's own buffer")
	}
}

func TestKnownVariantHasNoRaw(t *testing.T) {
	var v ShapeCollectionShapesValue
	if err := json.Unmarshal([]byte("{\"shapeType\":\"circle\",\"radius\":1}"), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.Raw() != nil {
		t.Errorf("Raw() = %s, want nil for a recognized variant", v.Raw())
	}
}

func TestNullUnionDecodesToTheZeroValue(t *testing.T) {
	var v ShapeCollectionShapesValue
	if err := json.Unmarshal([]byte("null"), &v); err != nil {
		t.Fatalf("null should decode as a no-op: %v", err)
	}
	if v.Value != nil || v.IsUnknownVariant() {
		t.Errorf("null produced Value=%v unknown=%v, want the zero union", v.Value, v.IsUnknownVariant())
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

const untypedVariantSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Mixed:
      oneOf:
        - { type: string }
        - type: object
          properties:
            q: { type: string }
        - description: anything at all
`

// TestE2E_UntypedUnionVariantStillDecodes covers a union member the analyzer
// cannot name: it still covers payloads the spec calls valid, and rejecting them
// would fail the whole response they arrive in.
func TestE2E_UntypedUnionVariantStillDecodes(t *testing.T) {
	files, _ := generateFromSpec(t, untypedVariantSpec, "mixedapi")
	runGeneratedWireTest(t, files, "untypedvariant", `package mixedapi

import (
	"encoding/json"
	"testing"
)

func TestInlineObjectVariantIsTyped(t *testing.T) {
	var m Mixed
	if err := json.Unmarshal([]byte(`+"`"+`{"q":"x"}`+"`"+`), &m); err != nil {
		t.Fatalf("a payload matching the inline object variant failed to decode: %v", err)
	}
	obj, ok := m.Value.(MixedVariant)
	if !ok {
		t.Fatalf("Value = %#v, want the named struct the inline variant declares", m.Value)
	}
	if obj.Q == nil || *obj.Q != "x" {
		t.Errorf("Q = %v, want x", obj.Q)
	}
}

// A variant that declares nothing has no Go type of its own, and the payloads it
// covers still have to decode.
func TestUntypedVariantDecodes(t *testing.T) {
	var m Mixed
	if err := json.Unmarshal([]byte(`+"`"+`[1,2,3]`+"`"+`), &m); err != nil {
		t.Fatalf("a payload matching only the untyped variant failed to decode: %v", err)
	}
	items, ok := m.Value.([]any)
	if !ok || len(items) != 3 {
		t.Errorf("Value = %#v, want the decoded array", m.Value)
	}
}

func TestTypedVariantStillWins(t *testing.T) {
	var m Mixed
	if err := json.Unmarshal([]byte(`+"`"+`"plain"`+"`"+`), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Value != "plain" {
		t.Errorf("Value = %#v, want the string variant", m.Value)
	}
}
`)
}
