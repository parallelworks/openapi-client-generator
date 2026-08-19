package generator

import (
	"strings"
	"testing"
)

const unionBaseSpec = `openapi: 3.1.0
info: { title: pets, version: "1" }
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: array
                items: { $ref: "#/components/schemas/Pet" }
components:
  schemas:
    PetBase:
      type: object
      properties:
        id: { type: string }
        name: { type: string }
        age: { type: integer }
      required: [id, name, age]
    Dog:
      allOf:
        - $ref: "#/components/schemas/PetBase"
        - type: object
          properties:
            kind: { type: string, enum: [dog] }
            goodBoy: { type: boolean }
          required: [kind, goodBoy]
    Cat:
      allOf:
        - $ref: "#/components/schemas/PetBase"
        - type: object
          properties:
            kind: { type: string, enum: [cat] }
            livesLeft: { type: integer }
          required: [kind, livesLeft]
    Pet:
      oneOf:
        - $ref: "#/components/schemas/Dog"
        - $ref: "#/components/schemas/Cat"
      discriminator:
        propertyName: kind
        mapping: { dog: "#/components/schemas/Dog", cat: "#/components/schemas/Cat" }
    Rock:
      type: object
      properties:
        kind: { type: string, enum: [rock] }
      required: [kind]
    Thing:
      oneOf:
        - $ref: "#/components/schemas/Dog"
        - $ref: "#/components/schemas/Rock"
      discriminator:
        propertyName: kind
        mapping: { dog: "#/components/schemas/Dog", rock: "#/components/schemas/Rock" }
`

// TestE2E_UnionBaseAccessor covers a discriminated oneOf whose variants all
// compose the same schema: the fields they share are readable off the union
// itself, without a type switch that goes stale when a variant is added.
func TestE2E_UnionBaseAccessor(t *testing.T) {
	files, _ := generateFromSpec(t, unionBaseSpec, "petsapi")

	var types string
	for _, f := range files {
		if f.Name == "types.go" {
			types = string(f.Content)
		}
	}
	if strings.Contains(types, "func (u Thing) Base()") {
		t.Error("Thing has no base shared by every variant, but got a Base accessor")
	}

	runGeneratedWireTest(t, files, "unionbase", `package petsapi

import (
	"encoding/json"
	"testing"
)

func TestBaseReadsTheSharedSchema(t *testing.T) {
	var pets []Pet
	payload := `+"`"+`[
		{"kind":"dog","id":"1","name":"Rex","age":4,"goodBoy":true},
		{"kind":"cat","id":"2","name":"Momo","age":7,"livesLeft":9}
	]`+"`"+`
	if err := json.Unmarshal([]byte(payload), &pets); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	want := []string{"Rex", "Momo"}
	for i, p := range pets {
		base := p.Base()
		if base == nil {
			t.Fatalf("pets[%d].Base() = nil, want the shared PetBase", i)
		}
		if base.Name != want[i] {
			t.Errorf("pets[%d].Base().Name = %q, want %q", i, base.Name, want[i])
		}
	}
	if pets[0].Base().Age != 4 {
		t.Errorf("pets[0].Base().Age = %d, want 4", pets[0].Base().Age)
	}
	if _, ok := pets[1].Value.(Cat); !ok {
		t.Errorf("pets[1].Value = %T, want Cat", pets[1].Value)
	}
}

func TestBaseIsNilForAnUnknownVariant(t *testing.T) {
	var p Pet
	if err := json.Unmarshal([]byte(`+"`"+`{"kind":"parrot","id":"3","name":"Polly"}`+"`"+`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !p.IsUnknownVariant() {
		t.Fatal("expected an unknown variant")
	}
	if p.Base() != nil {
		t.Errorf("Base() = %+v, want nil for an unknown variant", p.Base())
	}
}

func TestBaseDoesNotAliasTheVariant(t *testing.T) {
	var p Pet
	if err := json.Unmarshal([]byte(`+"`"+`{"kind":"dog","id":"1","name":"Rex","age":4,"goodBoy":true}`+"`"+`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	p.Base().Name = "Fido"
	if p.Value.(Dog).Name != "Rex" {
		t.Error("writing through Base() changed the value the union holds")
	}
}
`)
}
