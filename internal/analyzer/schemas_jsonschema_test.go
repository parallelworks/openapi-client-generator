package analyzer

import (
	"strings"
	"testing"
)

const jsonSchemaKeywordSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Bags:
      type: object
      properties:
        extensions:
          type: object
          patternProperties:
            "^x-": { type: string }
        records:
          type: object
          patternProperties:
            "^rec-": { $ref: "#/components/schemas/Record" }
        mixed:
          type: object
          patternProperties:
            "^s-": { type: string }
            "^n-": { type: integer }
        explicit:
          type: object
          patternProperties:
            "^x-": { type: string }
          additionalProperties: { type: integer }
        pair:
          type: array
          prefixItems: [{ type: string }, { type: integer }]
    Record:
      type: object
      properties:
        id: { type: string }
`

// One pattern types every key it admits, which is the common shape: a bag of
// extension keys that all hold the same thing.
func TestPatternProperties_OnePatternTypesTheMap(t *testing.T) {
	_, typeMap := analyzeSpec(t, jsonSchemaKeywordSpec)

	fields := map[string]string{}
	for _, f := range typeMap["Bags"].Fields {
		fields[f.JSONName] = f.Type
	}

	if got := fields["extensions"]; got != "map[string]string" {
		t.Errorf("extensions = %q, want map[string]string", got)
	}
	if got := fields["records"]; got != "map[string]Record" {
		t.Errorf("records = %q, want map[string]Record", got)
	}
	// Several patterns disagree about what a key holds, and a map has one value
	// type.
	if got := fields["mixed"]; got != "map[string]any" {
		t.Errorf("mixed = %q, want map[string]any", got)
	}
	// An explicit additionalProperties still wins: it is the stated answer for
	// keys no pattern claims.
	if got := fields["explicit"]; got != "map[string]int64" {
		t.Errorf("explicit = %q, want map[string]int64", got)
	}
}

// A keyword the generator reads and cannot express should say so rather than
// leaving the author to find it in the output.
func TestUnsupportedKeywords_WarnOncePerSpec(t *testing.T) {
	pkg, _ := analyzeSpec(t, jsonSchemaKeywordSpec)

	for _, want := range []string{"prefixItems", "patternProperties with more than one pattern"} {
		var count int
		for _, w := range pkg.Warnings {
			if strings.Contains(w, want) {
				count++
			}
		}
		if count != 1 {
			t.Errorf("warnings mentioning %q = %d, want 1: %v", want, count, pkg.Warnings)
		}
	}
}
