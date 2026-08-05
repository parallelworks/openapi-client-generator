package analyzer

import "testing"

// TestEnumConstName pins how an enum value becomes a Go constant name. The
// punctuation rows are the ones that used to collapse onto a single identifier
// (issue #15): every member of a comparison-operator enum sanitized to the same
// name, so the generated const block did not compile.
func TestEnumConstName(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
		raw      string
		want     string
	}{
		// The reported case: an enum of relational operators.
		{"equal", "RelationalOperator", "=", "RelationalOperatorEqual"},
		{"angle not equal", "RelationalOperator", "<>", "RelationalOperatorNotEqual"},
		{"greater", "RelationalOperator", ">", "RelationalOperatorGreaterThan"},
		{"less", "RelationalOperator", "<", "RelationalOperatorLessThan"},
		{"greater or equal", "RelationalOperator", ">=", "RelationalOperatorGreaterThanOrEqual"},
		{"less or equal", "RelationalOperator", "<=", "RelationalOperatorLessThanOrEqual"},
		{"bang not equal", "RelationalOperator", "!=", "RelationalOperatorNotEqual"},
		{"double equal", "RelationalOperator", "==", "RelationalOperatorEqual"},

		// Punctuation with no operator spelling falls back to rune names.
		{"star", "Wildcard", "*", "WildcardStar"},
		{"slash", "Sep", "/", "SepSlash"},
		{"double colon", "Sep", "::", "SepColonColon"},
		{"arrow", "Dir", "->", "DirMinusGreater"},
		{"empty", "Blank", "", "BlankEmpty"},

		// Alphanumeric values keep the ordinary naming; punctuation inside them is
		// still just a word separator.
		{"word", "Status", "active", "StatusActive"},
		{"hyphenated", "Status", "in-progress", "StatusInProgress"},
		{"mixed", "Status", "n/a", "StatusNA"},
		{"initialism", "Format", "json", "FormatJSON"},
		{"numeric value", "Version", "2", "Version2"},

		// A value made of runes with no name still yields something; the caller's
		// uniquing scope resolves any collision.
		{"unnameable", "Sym", "€", "Sym"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := enumConstName(tt.typeName, tt.raw); got != tt.want {
				t.Errorf("enumConstName(%q, %q) = %q, want %q", tt.typeName, tt.raw, got, tt.want)
			}
		})
	}
}

// TestConvertEnum_SymbolicValuesGetDistinctNames checks the whole enum path: the
// six operator values must produce six distinct, non-numeric constant names.
func TestConvertEnum_SymbolicValuesGetDistinctNames(t *testing.T) {
	_, typeMap := analyzeSpec(t, `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    RelationalOperator:
      type: string
      enum: ["=", "<>", ">", "<", ">=", "<="]
`)

	td := typeMap["RelationalOperator"]
	if td == nil {
		t.Fatal("RelationalOperator type not found")
	}
	if len(td.EnumValues) != 6 {
		t.Fatalf("enum values = %d, want 6", len(td.EnumValues))
	}

	want := map[string]string{
		"RelationalOperatorEqual":              `"="`,
		"RelationalOperatorNotEqual":           `"<>"`,
		"RelationalOperatorGreaterThan":        `">"`,
		"RelationalOperatorLessThan":           `"<"`,
		"RelationalOperatorGreaterThanOrEqual": `">="`,
		"RelationalOperatorLessThanOrEqual":    `"<="`,
	}
	seen := make(map[string]bool, len(td.EnumValues))
	for _, ev := range td.EnumValues {
		if seen[ev.Name] {
			t.Errorf("duplicate constant name %q", ev.Name)
		}
		seen[ev.Name] = true
		literal, ok := want[ev.Name]
		if !ok {
			t.Errorf("unexpected constant %q = %s", ev.Name, ev.Literal)
			continue
		}
		if ev.Literal != literal {
			t.Errorf("%s = %s, want %s", ev.Name, ev.Literal, literal)
		}
	}
}
