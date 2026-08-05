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
