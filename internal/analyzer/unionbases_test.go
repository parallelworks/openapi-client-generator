package analyzer

import "testing"

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
