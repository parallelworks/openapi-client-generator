package analyzer

import "testing"

const fieldCollisionSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Thing:
      type: object
      properties:
        user-id: { type: string }
        user_id: { type: string }
        userId: { type: string }
        keep: { type: string }
    Composed:
      allOf:
        - $ref: "#/components/schemas/Base"
        - type: object
          properties:
            base: { type: string }
    Base:
      type: object
      properties:
        id: { type: string }
    Bagged:
      type: object
      properties:
        additionalProperties: { type: string }
      additionalProperties: { type: integer }
`

// Property names that differ on the wire can normalize to one Go identifier,
// which the compiler rejects as a redeclaration.
func TestFieldNames_CollisionsAreNumbered(t *testing.T) {
	_, typeMap := analyzeSpec(t, fieldCollisionSpec)

	thing := typeMap["Thing"]
	if thing == nil {
		t.Fatal("Thing not found")
	}
	var names, tags []string
	for _, f := range thing.Fields {
		names = append(names, f.Name)
		tags = append(tags, f.JSONName)
	}
	want := []string{"UserID", "UserID2", "UserID3", "Keep"}
	if len(names) != len(want) {
		t.Fatalf("fields = %v, want %v", names, want)
	}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("field %d = %q, want %q", i, names[i], w)
		}
	}
	// Only the Go name moves; each field keeps the property it came from.
	wantTags := []string{"user-id", "user_id", "userId", "keep"}
	for i, w := range wantTags {
		if tags[i] != w {
			t.Errorf("field %d tag = %q, want %q", i, tags[i], w)
		}
	}
}

// An embedded type from allOf shares the struct's namespace with the properties
// merged beside it.
func TestFieldNames_EmbeddedTypeCollidesWithAProperty(t *testing.T) {
	_, typeMap := analyzeSpec(t, fieldCollisionSpec)

	composed := typeMap["Composed"]
	if composed == nil {
		t.Fatal("Composed not found")
	}
	if len(composed.Fields) != 2 {
		t.Fatalf("fields = %+v, want the embed and the property", composed.Fields)
	}
	if composed.Fields[0].Name != "Base" || !composed.Fields[0].Embedded {
		t.Errorf("field 0 = %+v, want the embedded Base", composed.Fields[0])
	}
	if composed.Fields[1].Name != "Base2" {
		t.Errorf("field 1 = %q, want Base2", composed.Fields[1].Name)
	}
}

// The synthetic catch-all is subject to the same rule, from the other side.
func TestFieldNames_CatchAllAvoidsADeclaredProperty(t *testing.T) {
	_, typeMap := analyzeSpec(t, fieldCollisionSpec)

	bagged := typeMap["Bagged"]
	if bagged == nil {
		t.Fatal("Bagged not found")
	}
	if len(bagged.Fields) != 2 {
		t.Fatalf("fields = %+v, want the property and the catch-all", bagged.Fields)
	}
	if bagged.Fields[0].Name != "AdditionalProperties" || bagged.Fields[1].Name != "AdditionalProperties2" {
		t.Errorf("fields = %q, %q", bagged.Fields[0].Name, bagged.Fields[1].Name)
	}
	if !bagged.Fields[1].CatchAll {
		t.Error("the second field should be the catch-all")
	}
}
