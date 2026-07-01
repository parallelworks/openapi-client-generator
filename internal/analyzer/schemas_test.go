package analyzer

import "testing"

// TestEnumConstLiteral pins the exact (goType, raw) -> (literal, ok) contract of
// enum constant rendering. Every past enum regression (leading-zero octal misparse,
// dropped 08/09, overflow reread as octal, NaN/Inf slipping through, string vs
// numeric quoting) is a row here, so a change that reintroduces one fails fast.
func TestEnumConstLiteral(t *testing.T) {
	tests := []struct {
		name   string
		goType string
		raw    string
		want   string
		wantOK bool
	}{
		// strings: always quoted, always representable.
		{"string plain", "string", "active", `"active"`, true},
		{"string empty", "string", "", `""`, true},
		{"string literal null", "string", "null", `"null"`, true},
		{"string hyphen", "string", "in-progress", `"in-progress"`, true},
		{"string with quote", "string", `a"b`, `"a\"b"`, true},
		{"string with newline", "string", "a\nb", `"a\nb"`, true},
		{"string unicode", "string", "café", `"café"`, true},

		// bool.
		{"bool true", "bool", "true", "true", true},
		{"bool false", "bool", "false", "false", true},
		{"bool numeric one", "bool", "1", "true", true},
		{"bool invalid", "bool", "yes", "", false},

		// int64: decimal stays decimal, 08/09 kept, hex/octal/binary/underscore via fallback.
		{"int decimal", "int64", "42", "42", true},
		{"int leading zero stays decimal", "int64", "010", "10", true},
		{"int 08 kept", "int64", "08", "8", true},
		{"int 09 kept", "int64", "09", "9", true},
		{"int negative", "int64", "-5", "-5", true},
		{"int hex", "int64", "0x1F", "31", true},
		{"int octal prefix", "int64", "0o17", "15", true},
		{"int binary", "int64", "0b101", "5", true},
		{"int underscore", "int64", "1_000", "1000", true},
		{"int64 overflow dropped", "int64", "99999999999999999999", "", false},
		{"int garbage dropped", "int64", "abc", "", false},

		// int32: range-checked; a leading-zero overflow is dropped, not reread as octal.
		{"int32 in range", "int32", "1000", "1000", true},
		{"int32 overflow dropped", "int32", "5000000000", "", false},
		{"int32 leading-zero overflow dropped not octal", "int32", "05000000000", "", false},
		{"int32 hex", "int32", "0x7F", "127", true},

		// float64: plain decimals, non-finite/unparseable dropped.
		{"float decimal", "float64", "1.5", "1.5", true},
		{"float integer valued plain", "float64", "1000000", "1000000", true},
		{"float scientific input plain output", "float64", "1e3", "1000", true},
		{"float NaN string dropped", "float64", "NaN", "", false},
		{"float Inf string dropped", "float64", "Inf", "", false},
		{"float +Inf dropped", "float64", "+Inf", "", false},
		{"float -Inf dropped", "float64", "-Inf", "", false},
		{"float yaml nan dropped", "float64", ".nan", "", false},
		{"float garbage dropped", "float64", "abc", "", false},

		// float32: range-checked.
		{"float32 in range", "float32", "1.5", "1.5", true},
		{"float32 overflow dropped", "float32", "1e40", "", false},

		// A non-const-able goType never produces a literal.
		{"non-constable dropped", "time.Time", "2020-01-01", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := enumConstLiteral(tt.goType, tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("enumConstLiteral(%q, %q) ok = %v, want %v (got %q)", tt.goType, tt.raw, ok, tt.wantOK, got)
			}
			if ok && got != tt.want {
				t.Errorf("enumConstLiteral(%q, %q) = %q, want %q", tt.goType, tt.raw, got, tt.want)
			}
		})
	}
}

func TestConstableType(t *testing.T) {
	for _, g := range []string{"string", "bool", "int32", "int64", "float32", "float64"} {
		if !constableType(g) {
			t.Errorf("constableType(%q) = false, want true", g)
		}
	}
	for _, g := range []string{"time.Time", "[]byte", "any", "map[string]string", "int", "uint64"} {
		if constableType(g) {
			t.Errorf("constableType(%q) = true, want false", g)
		}
	}
}
