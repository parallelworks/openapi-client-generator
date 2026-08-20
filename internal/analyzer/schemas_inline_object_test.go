package analyzer

import (
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
)

const inlineObjectSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Outer:
      type: object
      properties:
        inner:
          type: object
          properties:
            a: { type: string }
            deeper:
              type: object
              properties:
                b: { type: integer }
          required: [a]
        tags:
          type: array
          items:
            type: object
            properties:
              name: { type: string }
        same:
          type: object
          properties:
            a: { type: string }
            deeper:
              type: object
              properties:
                b: { type: integer }
          required: [a]
        titled:
          type: object
          title: Marker
          properties:
            id: { type: string }
        bag:
          type: object
          additionalProperties: { type: string }
`

func TestInlineObject_PropertiesStayTyped(t *testing.T) {
	_, typeMap := analyzeSpec(t, inlineObjectSpec)

	outer := typeMap["Outer"]
	if outer == nil {
		t.Fatal("Outer not found")
	}
	byJSON := make(map[string]*ir.Field, len(outer.Fields))
	for _, f := range outer.Fields {
		byJSON[f.JSONName] = f
	}

	if got := byJSON["inner"].Type; got != "*OuterInner" {
		t.Errorf("inner type = %q, want *OuterInner", got)
	}
	if got := byJSON["tags"].Type; got != "[]OuterTagsItem" {
		t.Errorf("tags type = %q, want []OuterTagsItem", got)
	}
	// An object with no properties still has nothing to declare fields from.
	if got := byJSON["bag"].Type; got != "map[string]string" {
		t.Errorf("bag type = %q, want map[string]string", got)
	}

	inner := typeMap["OuterInner"]
	if inner == nil {
		t.Fatal("OuterInner not synthesized")
	}
	if len(inner.Fields) != 2 {
		t.Fatalf("OuterInner fields = %+v, want a and deeper", inner.Fields)
	}
	if inner.Fields[0].Type != "string" || !inner.Fields[0].Required {
		t.Errorf("OuterInner.A = %+v, want a required string", inner.Fields[0])
	}
	// Nesting keeps going: the object inside the object is named too.
	if inner.Fields[1].Type != "*OuterInnerDeeper" {
		t.Errorf("OuterInner.Deeper = %q, want *OuterInnerDeeper", inner.Fields[1].Type)
	}
	if typeMap["OuterInnerDeeper"] == nil {
		t.Error("OuterInnerDeeper not synthesized")
	}
}

// Two properties declaring one shape share a type rather than generating a
// duplicate under each property's name.
func TestInlineObject_IdenticalShapesShareAType(t *testing.T) {
	_, typeMap := analyzeSpec(t, inlineObjectSpec)

	outer := typeMap["Outer"]
	var inner, same string
	for _, f := range outer.Fields {
		switch f.JSONName {
		case "inner":
			inner = f.Type
		case "same":
			same = f.Type
		}
	}
	if inner != same {
		t.Errorf("inner = %q and same = %q, want one type for one shape", inner, same)
	}
	if typeMap["OuterSame"] != nil {
		t.Error("a second type was generated for a shape that already had one")
	}
}

// A titled schema names itself, so renaming the property that reaches it first
// does not rename the generated type.
func TestInlineObject_TitleNamesTheType(t *testing.T) {
	_, typeMap := analyzeSpec(t, inlineObjectSpec)

	if typeMap["Marker"] == nil {
		t.Error("a titled inline object should be named for its title")
	}
	if typeMap["OuterTitled"] != nil {
		t.Error("the title was ignored in favor of the property name")
	}
}
