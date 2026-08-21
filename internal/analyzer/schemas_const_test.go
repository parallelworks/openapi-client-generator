package analyzer

import (
	"strings"
	"testing"
)

const constSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Thing:
      type: object
      properties:
        kind: { const: dog }
        count: { const: 3 }
        ratio: { const: 1.5 }
        enabled: { const: true }
        stated: { type: string, const: v2 }
        shaped:
          const: { a: 1 }
        conditional:
          type: object
          properties: { a: { type: string } }
          if: { properties: { a: { const: x } } }
          then: { required: [b] }
`

// A const says what the value is, which says what type it has. 3.1 specs pin a
// discriminator this way, and the property was resolving to any.
func TestConst_ImpliesTheType(t *testing.T) {
	_, typeMap := analyzeSpec(t, constSpec)

	byJSON := map[string]string{}
	for _, f := range typeMap["Thing"].Fields {
		byJSON[f.JSONName] = f.Type
	}

	// Optional properties are pointers, as everywhere else.
	for prop, want := range map[string]string{
		"kind":    "*string",
		"count":   "*int64",
		"ratio":   "*float64",
		"enabled": "*bool",
		// A schema that states its own type keeps it.
		"stated": "*string",
	} {
		if byJSON[prop] != want {
			t.Errorf("%s = %q, want %q", prop, byJSON[prop], want)
		}
	}

	// A const that is not a scalar says nothing a Go type can carry on its own.
	if byJSON["shaped"] != "any" {
		t.Errorf("shaped = %q, want any", byJSON["shaped"])
	}
}

// if/then/else makes a shape depend on a value, which is a validation rule
// rather than a type, and was dropped without a word while its sibling keyword
// dependentSchemas warned.
func TestConditionalSchemas_Warn(t *testing.T) {
	pkg, _ := analyzeSpec(t, constSpec)

	var found int
	for _, w := range pkg.Warnings {
		if strings.Contains(w, "if/then/else") {
			found++
		}
	}
	if found != 1 {
		t.Errorf("if/then/else warnings = %d, want 1: %v", found, pkg.Warnings)
	}
}
