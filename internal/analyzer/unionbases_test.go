package analyzer

import (
	"reflect"
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
)

const sharedBaseSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    PetBase:
      type: object
      properties:
        id: { type: string }
        name: { type: string }
      required: [id, name]
    Timestamps:
      type: object
      properties:
        createdAt: { type: string, format: date-time }
    Dog:
      allOf:
        - $ref: "#/components/schemas/PetBase"
        - type: object
          properties:
            kind: { type: string }
            goodBoy: { type: boolean }
          required: [kind, goodBoy]
    Cat:
      allOf:
        - $ref: "#/components/schemas/PetBase"
        - $ref: "#/components/schemas/Timestamps"
        - type: object
          properties:
            kind: { type: string }
            livesLeft: { type: integer }
          required: [kind, livesLeft]
    Rock:
      type: object
      properties:
        kind: { type: string }
      required: [kind]
    Pet:
      oneOf:
        - $ref: "#/components/schemas/Dog"
        - $ref: "#/components/schemas/Cat"
      discriminator:
        propertyName: kind
        mapping: { dog: "#/components/schemas/Dog", cat: "#/components/schemas/Cat" }
    Thing:
      oneOf:
        - $ref: "#/components/schemas/Dog"
        - $ref: "#/components/schemas/Rock"
      discriminator:
        propertyName: kind
        mapping: { dog: "#/components/schemas/Dog", rock: "#/components/schemas/Rock" }
    Untagged:
      oneOf:
        - $ref: "#/components/schemas/Dog"
        - $ref: "#/components/schemas/Cat"
`

func TestUnionBase_SharedByEveryVariant(t *testing.T) {
	_, typeMap := analyzeSpec(t, sharedBaseSpec)

	pet := typeMap["Pet"]
	if pet == nil {
		t.Fatal("Pet type not found")
	}
	if pet.BaseType != "PetBase" {
		t.Errorf("Pet.BaseType = %q, want PetBase", pet.BaseType)
	}
}

// Timestamps is embedded by Cat alone, so it is not a base of the union.
func TestUnionBase_IgnoresBaseOnlySomeVariantsCompose(t *testing.T) {
	_, typeMap := analyzeSpec(t, sharedBaseSpec)

	thing := typeMap["Thing"]
	if thing == nil {
		t.Fatal("Thing type not found")
	}
	if thing.BaseType != "" {
		t.Errorf("Thing.BaseType = %q, want empty: Rock composes nothing", thing.BaseType)
	}
}

func TestUnionBase_NeedsNoDiscriminator(t *testing.T) {
	_, typeMap := analyzeSpec(t, sharedBaseSpec)

	untagged := typeMap["Untagged"]
	if untagged == nil {
		t.Fatal("Untagged type not found")
	}
	if untagged.BaseType != "PetBase" {
		t.Errorf("Untagged.BaseType = %q, want PetBase", untagged.BaseType)
	}
}

const ambiguousBaseSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Named:
      type: object
      properties:
        name: { type: string }
    Owned:
      type: object
      properties:
        owner: { type: string }
    A:
      allOf:
        - $ref: "#/components/schemas/Named"
        - $ref: "#/components/schemas/Owned"
    B:
      allOf:
        - $ref: "#/components/schemas/Named"
        - $ref: "#/components/schemas/Owned"
    Either:
      oneOf:
        - $ref: "#/components/schemas/A"
        - $ref: "#/components/schemas/B"
`

// Two shared bases give no single one to expose, so the union keeps its old shape.
func TestUnionBase_AmbiguousBaseIsLeftAlone(t *testing.T) {
	_, typeMap := analyzeSpec(t, ambiguousBaseSpec)

	either := typeMap["Either"]
	if either == nil {
		t.Fatal("Either type not found")
	}
	if either.BaseType != "" {
		t.Errorf("Either.BaseType = %q, want empty", either.BaseType)
	}
}

const inlinedSharedSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Dog:
      type: object
      properties:
        id: { type: string }
        name: { type: string }
        age: { type: integer }
        kind: { type: string, enum: [dog] }
        goodBoy: { type: boolean }
      required: [id, name, age, kind, goodBoy]
    Cat:
      type: object
      properties:
        id: { type: string }
        name: { type: string }
        age: { type: integer }
        kind: { type: string, enum: [cat] }
        livesLeft: { type: integer }
      required: [id, name, age, kind, livesLeft]
    Pet:
      oneOf:
        - $ref: "#/components/schemas/Dog"
        - $ref: "#/components/schemas/Cat"
      discriminator:
        propertyName: kind
        mapping: { dog: "#/components/schemas/Dog", cat: "#/components/schemas/Cat" }
`

// A spec that inlines the shared properties instead of composing them through
// allOf describes the same thing, and generators that flatten composition emit
// nothing else.
func TestUnionBase_SynthesizedFromInlinedProperties(t *testing.T) {
	_, typeMap := analyzeSpec(t, inlinedSharedSpec)

	pet := typeMap["Pet"]
	if pet == nil {
		t.Fatal("Pet type not found")
	}
	if pet.BaseType != "PetBase" {
		t.Fatalf("Pet.BaseType = %q, want PetBase", pet.BaseType)
	}
	if pet.BaseEmbedded {
		t.Error("Pet.BaseEmbedded = true, want false: the variants declare the properties themselves")
	}

	base := typeMap["PetBase"]
	if base == nil {
		t.Fatal("synthesized PetBase not found in package types")
	}
	if base.Kind != ir.TypeKindStruct {
		t.Errorf("PetBase kind = %v, want struct", base.Kind)
	}
	var got []string
	for _, f := range base.Fields {
		got = append(got, f.Name+" "+f.Type)
	}
	want := []string{"ID string", "Name string", "Age int64"}
	if len(got) != len(want) {
		t.Fatalf("PetBase fields = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("PetBase field %d = %q, want %q", i, got[i], w)
		}
	}
}

func TestUnionBase_EmbeddedBaseWinsOverSynthesis(t *testing.T) {
	_, typeMap := analyzeSpec(t, sharedBaseSpec)

	pet := typeMap["Pet"]
	if pet == nil {
		t.Fatal("Pet type not found")
	}
	if !pet.BaseEmbedded {
		t.Error("Pet.BaseEmbedded = false, want true: the variants compose PetBase")
	}
	if typeMap["PetBase"] == nil {
		t.Error("the spec's own PetBase should be the base, not a synthesized copy")
	}
}

const partiallySharedSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Dog:
      type: object
      properties:
        id: { type: string }
        age: { type: integer }
        kind: { type: string }
      required: [id, age, kind]
    Cat:
      type: object
      properties:
        id: { type: string }
        age: { type: string }
        name: { type: string }
        kind: { type: string }
      required: [id, age, kind]
    Pet:
      oneOf:
        - $ref: "#/components/schemas/Dog"
        - $ref: "#/components/schemas/Cat"
      discriminator:
        propertyName: kind
        mapping: { dog: "#/components/schemas/Dog", cat: "#/components/schemas/Cat" }
`

// age is an integer in one variant and a string in the other, and name is absent
// from one, so neither is a property the variants agree on.
func TestUnionBase_SynthesizesOnlyIdenticalProperties(t *testing.T) {
	_, typeMap := analyzeSpec(t, partiallySharedSpec)

	pet := typeMap["Pet"]
	if pet == nil {
		t.Fatal("Pet type not found")
	}
	base := typeMap[pet.BaseType]
	if base == nil {
		t.Fatalf("Pet.BaseType = %q, which is not a generated type", pet.BaseType)
	}
	if len(base.Fields) != 1 || base.Fields[0].Name != "ID" {
		t.Errorf("%s fields = %+v, want ID alone", base.Name, base.Fields)
	}
}

const discriminatorOnlySpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Dog:
      type: object
      properties:
        kind: { type: string }
        goodBoy: { type: boolean }
      required: [kind, goodBoy]
    Cat:
      type: object
      properties:
        kind: { type: string }
        livesLeft: { type: integer }
      required: [kind, livesLeft]
    Pet:
      oneOf:
        - $ref: "#/components/schemas/Dog"
        - $ref: "#/components/schemas/Cat"
      discriminator:
        propertyName: kind
        mapping: { dog: "#/components/schemas/Dog", cat: "#/components/schemas/Cat" }
`

// The discriminator is how the variants differ, and a spec that spells the base
// out keeps it out of the shared schema, so it is not a base of its own.
func TestUnionBase_DiscriminatorAloneIsNoBase(t *testing.T) {
	_, typeMap := analyzeSpec(t, discriminatorOnlySpec)

	pet := typeMap["Pet"]
	if pet == nil {
		t.Fatal("Pet type not found")
	}
	if pet.BaseType != "" {
		t.Errorf("Pet.BaseType = %q, want empty", pet.BaseType)
	}
}

const takenBaseNameSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    PetBase:
      type: object
      properties:
        unrelated: { type: string }
    Dog:
      type: object
      properties:
        id: { type: string }
        kind: { type: string }
      required: [id, kind]
    Cat:
      type: object
      properties:
        id: { type: string }
        kind: { type: string }
      required: [id, kind]
    Pet:
      oneOf:
        - $ref: "#/components/schemas/Dog"
        - $ref: "#/components/schemas/Cat"
      discriminator:
        propertyName: kind
        mapping: { dog: "#/components/schemas/Dog", cat: "#/components/schemas/Cat" }
`

func TestUnionBase_SynthesizedNameAvoidsTheSpecsOwn(t *testing.T) {
	_, typeMap := analyzeSpec(t, takenBaseNameSpec)

	pet := typeMap["Pet"]
	if pet == nil {
		t.Fatal("Pet type not found")
	}
	if pet.BaseType == "" || pet.BaseType == "PetBase" {
		t.Fatalf("Pet.BaseType = %q, want a name the spec's own PetBase does not hold", pet.BaseType)
	}
	if base := typeMap[pet.BaseType]; base == nil || len(base.Fields) != 1 {
		t.Errorf("%s = %+v, want the synthesized base holding id", pet.BaseType, base)
	}
	if spec := typeMap["PetBase"]; spec == nil || len(spec.Fields) != 1 || spec.Fields[0].Name != "Unrelated" {
		t.Errorf("the spec's PetBase was overwritten: %+v", spec)
	}
}

const nullableStructSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Person:
      type: object
      properties:
        id: { type: string }
        name: { type: string }
    MaybePerson:
      anyOf:
        - $ref: "#/components/schemas/Person"
        - type: "null"
    PersonOrName:
      anyOf:
        - $ref: "#/components/schemas/Person"
        - type: string
`

// One variant shares every property with itself, and a variant with no fields at
// all shares none, so neither shape has a base to expose.
func TestUnionBase_NeedsTwoStructVariants(t *testing.T) {
	_, typeMap := analyzeSpec(t, nullableStructSpec)

	for _, name := range []string{"MaybePerson", "PersonOrName"} {
		td := typeMap[name]
		if td == nil {
			continue
		}
		if td.BaseType != "" {
			t.Errorf("%s.BaseType = %q, want empty", name, td.BaseType)
		}
	}
}

// sharedFields copies each field it keeps with *f, which is a complete copy only
// while ir.Field holds nothing by reference. A member added later that the copy
// would alias has to be copied deliberately, and the aliasing is silent until
// something writes through it.
func TestSharedFieldsCopyStaysComplete(t *testing.T) {
	field := reflect.TypeFor[ir.Field]()
	for i := range field.NumField() {
		member := field.Field(i)
		switch member.Type.Kind() {
		case reflect.String, reflect.Bool:
		default:
			t.Errorf("ir.Field.%s is a %s, which sharedFields now aliases into the synthesized base", member.Name, member.Type.Kind())
		}
	}
}
