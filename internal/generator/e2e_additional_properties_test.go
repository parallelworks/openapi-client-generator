package generator

import (
	"strings"
	"testing"
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
    Sealed:
      type: object
      properties:
        only: { type: string }
      additionalProperties: false
    Base:
      type: object
      properties:
        id: { type: string }
    Composed:
      allOf:
        - $ref: "#/components/schemas/Base"
        - type: object
          properties:
            name: { type: string }
      additionalProperties: true
    NullableBase:
      anyOf: [{ $ref: "#/components/schemas/Base" }, { type: "null" }]
    ComposedThroughAlias:
      allOf:
        - $ref: "#/components/schemas/NullableBase"
        - type: object
          properties:
            note: { type: string }
      additionalProperties: true
    Parent:
      type: object
      properties:
        id: { type: string }
      additionalProperties: true
    Child:
      allOf:
        - $ref: "#/components/schemas/Parent"
        - type: object
          properties:
            name: { type: string }
      additionalProperties: true
    GrandChild:
      allOf:
        - $ref: "#/components/schemas/Child"
        - type: object
          properties:
            depth: { type: integer }
      additionalProperties: true
    TwinA:
      type: object
      properties:
        a: { type: string }
      additionalProperties: true
    TwinB:
      type: object
      properties:
        b: { type: string }
      additionalProperties: true
    TwoParents:
      allOf:
        - $ref: "#/components/schemas/TwinA"
        - $ref: "#/components/schemas/TwinB"
        - type: object
          properties:
            own: { type: string }
    Circle:
      type: object
      properties:
        radius: { type: number }
    Shape:
      oneOf:
        - $ref: "#/components/schemas/Circle"
        - type: string
    Tagged:
      allOf:
        - $ref: "#/components/schemas/Shape"
        - type: object
          properties:
            label: { type: string }
      additionalProperties: true
    LabeledShape:
      allOf:
        - $ref: "#/components/schemas/Shape"
        - type: object
          properties:
            title: { type: string }
    StrictChild:
      allOf:
        - $ref: "#/components/schemas/Parent"
        - type: object
          properties:
            name: { type: string }
      additionalProperties:
        type: string
    Node:
      allOf:
        - $ref: "#/components/schemas/NodeBase"
        - type: object
          properties:
            label: { type: string }
      additionalProperties: true
    NodeBase:
      allOf:
        - $ref: "#/components/schemas/Node"
      additionalProperties: true
`

const additionalPropertiesRuntimeTest = `package petsapi

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

// encoding/json falls back to a case-insensitive tag match, so a differently
// cased declared property must not also be collected as an unknown one -- that
// would emit the same property twice on the way back out.
func TestDifferentlyCasedDeclaredPropertyIsNotDuplicated(t *testing.T) {
	var p Pet
	if err := json.Unmarshal([]byte(` + "`" + `{"Name":"rex","extra":1}` + "`" + `), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Name != "rex" {
		t.Errorf("Name = %q, want rex", p.Name)
	}
	if _, ok := p.AdditionalProperties["Name"]; ok {
		t.Fatalf("declared property leaked into AdditionalProperties: %v", p.AdditionalProperties)
	}

	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("round trip = %s, want exactly name and extra", out)
	}
}

// A decode must not carry over values from a previous one.
func TestDecodeResetsAdditionalProperties(t *testing.T) {
	p := Pet{AdditionalProperties: map[string]any{"stale": true}}
	if err := json.Unmarshal([]byte(` + "`" + `{"name":"rex","fresh":1}` + "`" + `), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := p.AdditionalProperties["stale"]; ok {
		t.Errorf("stale additional property survived: %v", p.AdditionalProperties)
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

// A composed schema collects undeclared properties just like a plain one, and
// the properties it inherits from the schema it embeds are not re-collected.
func TestComposedSchemaKeepsUnknownKeys(t *testing.T) {
	var c Composed
	if err := json.Unmarshal([]byte(` + "`" + `{"id":"x","name":"n","extra":"kept"}` + "`" + `), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.ID == nil || *c.ID != "x" {
		t.Errorf("inherited property lost: %v", c.ID)
	}
	if c.Name == nil || *c.Name != "n" {
		t.Errorf("declared property lost: %v", c.Name)
	}
	if got := c.AdditionalProperties["extra"]; got != "kept" {
		t.Errorf("AdditionalProperties[extra] = %v, want kept", got)
	}
	if _, ok := c.AdditionalProperties["id"]; ok {
		t.Error("a property inherited from the embedded schema landed in the catch-all")
	}

	out, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if n := strings.Count(string(out), ` + "`" + `"id"` + "`" + `); n != 1 {
		t.Errorf("id emitted %d times: %s", n, out)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	for key, want := range map[string]any{"id": "x", "name": "n", "extra": "kept"} {
		if got[key] != want {
			t.Errorf("round trip[%s] = %v, want %v", key, got[key], want)
		}
	}
}

// One undeclared property of the wrong type must not cost the caller the whole
// response; the declared fields are what consumers depend on.
func TestOffTypeExtraDoesNotFailTheDecode(t *testing.T) {
	var l Labels
	if err := json.Unmarshal([]byte(` + "`" + `{"owner":"me","count":3,"env":"prod"}` + "`" + `), &l); err != nil {
		t.Fatalf("one off-type extra failed the whole decode: %v", err)
	}
	if l.Owner == nil || *l.Owner != "me" {
		t.Errorf("Owner = %v, want me", l.Owner)
	}
	if l.AdditionalProperties["env"] != "prod" {
		t.Errorf("AdditionalProperties[env] = %q, want prod", l.AdditionalProperties["env"])
	}
	if _, ok := l.AdditionalProperties["count"]; ok {
		t.Error("a property that does not match the declared value type was kept anyway")
	}
}

// An embedded schema with additionalProperties of its own must not let its
// promoted unmarshaler sweep the composed type's declared fields into the
// embedded catch-all.
func TestEmbeddedCatchAllDoesNotSwallowDeclaredFields(t *testing.T) {
	var c Child
	if err := json.Unmarshal([]byte(` + "`" + `{"id":"x","name":"n","extra":"e"}` + "`" + `), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.ID == nil || *c.ID != "x" {
		t.Errorf("inherited property lost: %v", c.ID)
	}
	if c.Name == nil || *c.Name != "n" {
		t.Errorf("declared property swallowed by the embedded catch-all: %v", c.Name)
	}
	if got := c.AdditionalProperties["extra"]; got != "e" {
		t.Errorf("AdditionalProperties[extra] = %v, want e", got)
	}
	if len(c.Parent.AdditionalProperties) != 0 {
		t.Errorf("embedded catch-all should stay empty, got %v", c.Parent.AdditionalProperties)
	}

	out, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{` + "`" + `"id"` + "`" + `, ` + "`" + `"name"` + "`" + `, ` + "`" + `"extra"` + "`" + `} {
		if n := strings.Count(string(out), key); n != 1 {
			t.Errorf("%s emitted %d times: %s", key, n, out)
		}
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	want := map[string]any{"id": "x", "name": "n", "extra": "e"}
	if len(got) != len(want) {
		t.Fatalf("round trip = %s, want %v", out, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("round trip[%s] = %v, want %v", k, got[k], v)
		}
	}
}

// The same swallowing, two levels deep: the grandparent's catch-all sits behind
// two promotions.
func TestEmbeddedCatchAllChainThroughGrandparent(t *testing.T) {
	var g GrandChild
	if err := json.Unmarshal([]byte(` + "`" + `{"id":"x","name":"n","depth":2,"extra":"e"}` + "`" + `), &g); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if g.ID == nil || *g.ID != "x" {
		t.Errorf("grandparent property lost: %v", g.ID)
	}
	if g.Name == nil || *g.Name != "n" {
		t.Errorf("parent property lost: %v", g.Name)
	}
	if g.Depth == nil || *g.Depth != 2 {
		t.Errorf("declared property lost: %v", g.Depth)
	}
	if got := g.AdditionalProperties["extra"]; got != "e" {
		t.Errorf("AdditionalProperties[extra] = %v, want e", got)
	}
	if len(g.Child.AdditionalProperties) != 0 || len(g.Child.Parent.AdditionalProperties) != 0 {
		t.Errorf("embedded catch-alls should stay empty, got %v and %v",
			g.Child.AdditionalProperties, g.Child.Parent.AdditionalProperties)
	}

	out, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{` + "`" + `"id"` + "`" + `, ` + "`" + `"name"` + "`" + `, ` + "`" + `"depth"` + "`" + `, ` + "`" + `"extra"` + "`" + `} {
		if n := strings.Count(string(out), key); n != 1 {
			t.Errorf("%s emitted %d times: %s", key, n, out)
		}
	}
}

// With two embedded catch-all parents Go promotes no marshalers at all (the
// selector is ambiguous), so without generated ones the behavior flips on how
// many parents a schema composes. The composed schema declares no
// additionalProperties of its own; the extras stay on the embedded parents.
func TestTwoEmbeddedCatchAllParents(t *testing.T) {
	var p TwoParents
	if err := json.Unmarshal([]byte(` + "`" + `{"a":"1","b":"2","own":"3","extra":"e"}` + "`" + `), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.A == nil || *p.A != "1" {
		t.Errorf("A = %v, want 1", p.A)
	}
	if p.B == nil || *p.B != "2" {
		t.Errorf("B = %v, want 2", p.B)
	}
	if p.Own == nil || *p.Own != "3" {
		t.Errorf("Own = %v, want 3", p.Own)
	}
	if got := p.TwinA.AdditionalProperties["extra"]; got != "e" {
		t.Errorf("TwinA.AdditionalProperties[extra] = %v, want e", got)
	}
	for _, key := range []string{"a", "b", "own"} {
		if _, ok := p.TwinA.AdditionalProperties[key]; ok {
			t.Errorf("declared property %q landed in TwinA's catch-all", key)
		}
		if _, ok := p.TwinB.AdditionalProperties[key]; ok {
			t.Errorf("declared property %q landed in TwinB's catch-all", key)
		}
	}

	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{` + "`" + `"a"` + "`" + `, ` + "`" + `"b"` + "`" + `, ` + "`" + `"own"` + "`" + `, ` + "`" + `"extra"` + "`" + `} {
		if n := strings.Count(string(out), key); n != 1 {
			t.Errorf("%s emitted %d times: %s", key, n, out)
		}
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	want := map[string]any{"a": "1", "b": "2", "own": "3", "extra": "e"}
	if len(got) != len(want) {
		t.Fatalf("round trip = %s, want %v", out, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("round trip[%s] = %v, want %v", k, got[k], v)
		}
	}
}

// A composed type embedding a union keeps its own fields: promotion would let
// the union's marshaler emit only the union value.
func TestEmbeddedUnionObjectVariant(t *testing.T) {
	var v Tagged
	if err := json.Unmarshal([]byte(` + "`" + `{"radius":1.5,"label":"L","extra":"e"}` + "`" + `), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.Label == nil || *v.Label != "L" {
		t.Errorf("Label = %v, want L", v.Label)
	}
	circle, ok := v.Shape.Value.(Circle)
	if !ok {
		t.Fatalf("Shape.Value = %T, want Circle", v.Shape.Value)
	}
	if circle.Radius == nil || *circle.Radius != 1.5 {
		t.Errorf("Radius = %v, want 1.5", circle.Radius)
	}
	if _, ok := v.AdditionalProperties["radius"]; ok {
		t.Errorf("union variant property leaked into AdditionalProperties: %v", v.AdditionalProperties)
	}

	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{` + "`" + `"radius"` + "`" + `, ` + "`" + `"label"` + "`" + `, ` + "`" + `"extra"` + "`" + `} {
		if n := strings.Count(string(out), key); n != 1 {
			t.Errorf("%s emitted %d times: %s", key, n, out)
		}
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	for k, want := range map[string]any{"radius": 1.5, "label": "L", "extra": "e"} {
		if got[k] != want {
			t.Errorf("round trip[%s] = %v, want %v", k, got[k], want)
		}
	}

	two := 2.0
	v.Shape.Value = Circle{Radius: &two}
	updated, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal updated: %v", err)
	}
	var re map[string]any
	if err := json.Unmarshal(updated, &re); err != nil {
		t.Fatalf("re-unmarshal updated: %v", err)
	}
	if re["radius"] != 2.0 {
		t.Errorf("stale catch-all copy overrode the updated union value: %s", updated)
	}
}

// A schema with no additionalProperties of its own still needs marshalers when
// it embeds a type that has them, or the promotion swallows its fields anyway.
func TestEmbeddedUnionWithoutOwnCatchAll(t *testing.T) {
	var s LabeledShape
	if err := json.Unmarshal([]byte(` + "`" + `{"radius":2,"title":"T"}` + "`" + `), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Title == nil || *s.Title != "T" {
		t.Errorf("Title = %v, want T", s.Title)
	}
	circle, ok := s.Shape.Value.(Circle)
	if !ok {
		t.Fatalf("Shape.Value = %T, want Circle", s.Shape.Value)
	}
	if circle.Radius == nil || *circle.Radius != 2 {
		t.Errorf("Radius = %v, want 2", circle.Radius)
	}

	out, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{` + "`" + `"radius"` + "`" + `, ` + "`" + `"title"` + "`" + `} {
		if n := strings.Count(string(out), key); n != 1 {
			t.Errorf("%s emitted %d times: %s", key, n, out)
		}
	}
}

// A scalar union value cannot be part of a JSON object, so marshaling fails
// with an error that names the embedded field.
func TestEmbeddedUnionScalarVariantFailsWithNamedError(t *testing.T) {
	label := "L"
	v := Tagged{Shape: Shape{Value: "not an object"}, Label: &label}
	if _, err := json.Marshal(v); err == nil || !strings.Contains(err.Error(), "Shape") {
		t.Errorf("marshal = %v, want an error naming the embedded Shape", err)
	}
}

// The composed type's catch-all is narrower than the embedded one, so an extra
// only the wider embedded map can hold must survive there.
func TestOffTypeExtraSurvivesInTheWiderEmbeddedCatchAll(t *testing.T) {
	var s StrictChild
	if err := json.Unmarshal([]byte(` + "`" + `{"id":"x","name":"n","note":"ok","count":3}` + "`" + `), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := s.AdditionalProperties["note"]; got != "ok" {
		t.Errorf("AdditionalProperties[note] = %v, want ok", got)
	}
	if _, ok := s.AdditionalProperties["count"]; ok {
		t.Errorf("off-type extra landed in the string catch-all: %v", s.AdditionalProperties)
	}
	if got := s.Parent.AdditionalProperties["count"]; got != float64(3) {
		t.Errorf("Parent.AdditionalProperties[count] = %v, want 3", got)
	}

	out, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{` + "`" + `"id"` + "`" + `, ` + "`" + `"name"` + "`" + `, ` + "`" + `"note"` + "`" + `, ` + "`" + `"count"` + "`" + `} {
		if n := strings.Count(string(out), key); n != 1 {
			t.Errorf("%s emitted %d times: %s", key, n, out)
		}
	}
}

// Mutually recursive allOf schemas embed each other; decoding must terminate
// instead of re-entering the same unmarshaler until the stack overflows.
func TestCyclicSchemasDecodeWithoutOverflowingTheStack(t *testing.T) {
	var node Node
	if err := json.Unmarshal([]byte(` + "`" + `{"label":"a","extra":"e"}` + "`" + `), &node); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if node.Label == nil || *node.Label != "a" {
		t.Errorf("Label = %v, want a", node.Label)
	}
	if got := node.AdditionalProperties["extra"]; got != "e" {
		t.Errorf("AdditionalProperties[extra] = %v, want e", got)
	}

	out, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{` + "`" + `"label"` + "`" + `, ` + "`" + `"extra"` + "`" + `} {
		if n := strings.Count(string(out), key); n != 1 {
			t.Errorf("%s emitted %d times: %s", key, n, out)
		}
	}
}

// The embedded schema is reached through an alias, so the catch-all still has to
// recognize the properties that alias promotes as already declared.
func TestPropertiesInheritedThroughAnAliasAreNotRecollected(t *testing.T) {
	var c ComposedThroughAlias
	if err := json.Unmarshal([]byte(` + "`" + `{"id":"x","note":"n","extra":"kept"}` + "`" + `), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := c.AdditionalProperties["id"]; ok {
		t.Error("a property promoted through an aliased embed landed in the catch-all")
	}

	out, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if n := strings.Count(string(out), ` + "`" + `"id"` + "`" + `); n != 1 {
		t.Errorf("id emitted %d times: %s", n, out)
	}
}
`

// TestE2E_AdditionalPropertiesRoundTrip generates a client for schemas that mix
// declared properties with additionalProperties, then compiles and RUNS a test
// proving unknown keys survive an unmarshal/marshal round trip instead of being
// dropped or emitted under a literal "-" key.
func TestE2E_AdditionalPropertiesRoundTrip(t *testing.T) {
	files, _ := generateFromSpec(t, additionalPropertiesSpec, "petsapi")
	runGeneratedWireTest(t, files, "additionalprops", additionalPropertiesRuntimeTest)
}

// additionalProperties: false forbids unknown properties, so no catch-all field
// (and no custom marshalers) may be generated for such a schema.
func TestE2E_AdditionalPropertiesFalseHasNoCatchAll(t *testing.T) {
	files, _ := generateFromSpec(t, additionalPropertiesSpec, "petsapi")

	var types string
	for _, f := range files {
		if f.Name == "types.go" {
			types = string(f.Content)
		}
	}
	sealed := types[strings.Index(types, "type Sealed struct"):]
	sealed = sealed[:strings.Index(sealed, "\n}")]
	if strings.Contains(sealed, "AdditionalProperties") {
		t.Errorf("Sealed got a catch-all field despite additionalProperties: false:\n%s", sealed)
	}
	if strings.Contains(types, "func (t Sealed) MarshalJSON") {
		t.Error("Sealed got additionalProperties marshalers despite additionalProperties: false")
	}
}
