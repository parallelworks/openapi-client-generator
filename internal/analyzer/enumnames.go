package analyzer

import (
	"strings"
	"unicode"

	naming "github.com/giraffesyo/openapi-go-naming"
)

// operatorWords names the multi-character operators that turn up as enum values
// in filter and comparison DSLs, so they read as one idea rather than as their
// spelled-out parts.
var operatorWords = map[string]string{
	"=":  "Equal",
	"==": "Equal",
	"!=": "NotEqual",
	"<>": "NotEqual",
	"<":  "LessThan",
	"<=": "LessThanOrEqual",
	">":  "GreaterThan",
	">=": "GreaterThanOrEqual",
	"&&": "And",
	"||": "Or",
}

// symbolWords names individual punctuation runes.
var symbolWords = map[rune]string{
	' ':  "Space",
	'!':  "Not",
	'"':  "Quote",
	'#':  "Hash",
	'$':  "Dollar",
	'%':  "Percent",
	'&':  "And",
	'\'': "Apostrophe",
	'(':  "OpenParen",
	')':  "CloseParen",
	'*':  "Star",
	'+':  "Plus",
	',':  "Comma",
	'-':  "Minus",
	'.':  "Dot",
	'/':  "Slash",
	':':  "Colon",
	';':  "Semicolon",
	'<':  "Less",
	'=':  "Equal",
	'>':  "Greater",
	'?':  "Question",
	'@':  "At",
	'[':  "OpenBracket",
	'\\': "Backslash",
	']':  "CloseBracket",
	'^':  "Caret",
	'_':  "Underscore",
	'`':  "Backtick",
	'{':  "OpenBrace",
	'|':  "Or",
	'}':  "CloseBrace",
	'~':  "Tilde",
}

// enumConstName builds the Go constant name for one member of an enum. Values
// made only of punctuation ("=", "<>") are spelled out, because sanitizing them
// leaves nothing to name the constant after and every member of such an enum
// would want the same identifier.
func enumConstName(typeName, raw string) string {
	if raw == "" {
		return naming.Exported(typeName + " Empty")
	}
	if !hasAlphanumeric(raw) {
		if words := punctuationWords(raw); words != "" {
			return naming.Exported(typeName + " " + words)
		}
	}
	return naming.Exported(typeName + " " + raw)
}

// hasAlphanumeric reports whether s carries at least one rune that survives
// conversion to a Go identifier.
func hasAlphanumeric(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	})
}

// punctuationWords spells a punctuation-only value as words, returning "" when
// any rune has no name to spell it with.
func punctuationWords(raw string) string {
	if words, ok := operatorWords[raw]; ok {
		return words
	}
	var b strings.Builder
	b.Grow(len(raw) * 8)
	for _, r := range raw {
		word, ok := symbolWords[r]
		if !ok {
			return ""
		}
		b.WriteString(word)
	}
	return b.String()
}
