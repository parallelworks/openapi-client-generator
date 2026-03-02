package naming

import (
	"strings"
	"unicode"
	"fmt"
)

// goAcronyms maps lowercase acronyms to their proper Go uppercase form.
var goAcronyms = map[string]string{
	"id":   "ID",
	"url":  "URL",
	"api":  "API",
	"http": "HTTP",
	"json": "JSON",
	"xml":  "XML",
	"sql":  "SQL",
	"ssh":  "SSH",
	"ssl":  "SSL",
	"tls":  "TLS",
	"tcp":  "TCP",
	"udp":  "UDP",
	"ip":   "IP",
	"io":   "IO",
	"html": "HTML",
	"css":  "CSS",
	"uri":  "URI",
}

// goReserved is the set of Go reserved words.
var goReserved = map[string]bool{
	"type": true, "range": true, "map": true, "func": true,
	"interface": true, "struct": true, "chan": true, "go": true,
	"select": true, "case": true, "default": true, "break": true,
	"continue": true, "for": true, "if": true, "else": true,
	"switch": true, "return": true, "var": true, "const": true,
	"import": true, "package": true, "defer": true, "fallthrough": true,
	"goto": true,
}

// Namer handles conversion of OpenAPI identifiers to Go identifiers
// and tracks used names to prevent collisions.
type Namer struct {
	used map[string]int
}

func NewNamer() *Namer {
	return &Namer{used: make(map[string]int)}
}

// splitIdentifier splits an identifier string into words. It splits on
// non-alphanumeric characters and on camelCase/PascalCase boundaries,
// while keeping consecutive uppercase runs (acronyms) together.
func splitIdentifier(s string) []string {
	var words []string
	runes := []rune(s)
	start := -1

	flush := func(end int) {
		if start >= 0 && start < end {
			words = append(words, string(runes[start:end]))
		}
		start = -1
	}

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush(i)
			continue
		}
		if start < 0 {
			start = i
			continue
		}
		prev := runes[i-1]
		// Transition from lowercase/digit to uppercase starts a new word.
		if unicode.IsUpper(r) && (unicode.IsLower(prev) || unicode.IsDigit(prev)) {
			flush(i)
			start = i
			continue
		}
		// Transition within an uppercase run to a lowercase letter:
		// e.g. "HTMLParser" → "HTML", "Parser". The new word starts at i-1
		// (the last uppercase letter before the lowercase).
		if unicode.IsLower(r) && unicode.IsUpper(prev) && i-1 > start {
			flush(i - 1)
			start = i - 1
			continue
		}
	}
	flush(len(runes))
	return words
}

// capitalizeWord capitalizes a word, applying Go acronym conventions.
func capitalizeWord(w string) string {
	lower := strings.ToLower(w)
	if acronym, ok := goAcronyms[lower]; ok {
		return acronym
	}
	runes := []rune(lower)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// ToGoName converts any OpenAPI identifier to a valid Go exported
// identifier in PascalCase.
func ToGoName(s string) string {
	if s == "" {
		return "Unknown"
	}

	words := splitIdentifier(s)
	if len(words) == 0 {
		return "Unknown"
	}

	var b strings.Builder
	for _, w := range words {
		b.WriteString(capitalizeWord(w))
	}

	result := b.String()
	if goReserved[strings.ToLower(result)] {
		result += "_"
	}
	return result
}

// ToGoFieldName converts any OpenAPI identifier to a valid Go exported
// identifier in PascalCase for use as a struct field name. Unlike ToGoName,
// it does not escape Go reserved words because struct fields are accessed
// via selectors (e.g., obj.Type) which never conflict with keywords.
func ToGoFieldName(s string) string {
	if s == "" {
		return "Unknown"
	}

	words := splitIdentifier(s)
	if len(words) == 0 {
		return "Unknown"
	}

	var b strings.Builder
	for _, w := range words {
		b.WriteString(capitalizeWord(w))
	}

	return b.String()
}

// ToGoParamName converts any OpenAPI identifier to a valid Go unexported
// identifier in camelCase.
func ToGoParamName(s string) string {
	if s == "" {
		return "unknown"
	}

	words := splitIdentifier(s)
	if len(words) == 0 {
		return "unknown"
	}

	var b strings.Builder
	for i, w := range words {
		if i == 0 {
			lower := strings.ToLower(w)
			// For the first word, only use the acronym form if it's not
			// the only word (to match Go convention for unexported names).
			if _, isAcronym := goAcronyms[lower]; isAcronym && len(words) > 1 {
				b.WriteString(lower)
			} else {
				b.WriteString(lower)
			}
		} else {
			b.WriteString(capitalizeWord(w))
		}
	}

	result := b.String()
	if goReserved[result] {
		result += "_"
	}
	return result
}

// RegisterName registers a name and returns it. If the name has already
// been registered, a numeric suffix is appended to make it unique.
func (n *Namer) RegisterName(name string) string {
	n.used[name]++
	count := n.used[name]
	if count == 1 {
		return name
	}
	unique := fmt.Sprintf("%s%d", name, count)
	// In the unlikely event the suffixed name also collides, keep incrementing.
	for n.used[unique] > 0 {
		count++
		unique = fmt.Sprintf("%s%d", name, count)
	}
	n.used[name] = count
	n.used[unique] = 1
	return unique
}
