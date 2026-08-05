package analyzer

import (
	"strings"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
)

// breakAliasCycles degrades to any every alias that can reach itself. A generated
// alias is a true Go alias (`type A = B`), which the compiler expands eagerly, so
// a cycle through one is an "invalid recursive type" no matter how many slices,
// pointers, or maps sit between the two ends. Only aliases can form such a cycle:
// a struct, enum, or union names a real definition that terminates the chain.
func breakAliasCycles(types []*ir.TypeDef) {
	aliases := make(map[string]*ir.TypeDef, len(types))
	for _, td := range types {
		if td != nil && td.Kind == ir.TypeKindAlias {
			aliases[td.Name] = td
		}
	}

	const (
		visiting = 1
		done     = 2
	)
	state := make(map[string]int, len(aliases))

	// Reports whether the alias named by name sits on a cycle, breaking the edge
	// that closes one.
	var walk func(name string) bool
	walk = func(name string) bool {
		td, ok := aliases[name]
		if !ok {
			return false
		}
		switch state[name] {
		case visiting:
			return true
		case done:
			return false
		}

		state[name] = visiting
		if target := aliasTarget(td.GoType); target != "" && walk(target) {
			td.GoType = "any"
		}
		state[name] = done
		return false
	}

	for _, td := range types {
		if td != nil && td.Kind == ir.TypeKindAlias {
			walk(td.Name)
		}
	}
}

// aliasTarget returns the named type an alias's Go type expression refers to,
// peeling the slice, pointer, and map wrappers that do not stop a Go alias from
// expanding. It returns "" for a builtin or a composite with no single referent.
func aliasTarget(goType string) string {
	for {
		switch {
		case strings.HasPrefix(goType, "[]"):
			goType = goType[2:]
		case strings.HasPrefix(goType, "*"):
			goType = goType[1:]
		case strings.HasPrefix(goType, "map["):
			end := strings.Index(goType, "]")
			if end < 0 {
				return ""
			}
			goType = goType[end+1:]
		default:
			return namedType(goType)
		}
	}
}

// namedType returns goType when it is a bare type name rather than a builtin or
// a qualified type from another package.
func namedType(goType string) string {
	if goType == "" || goType == "any" || strings.ContainsAny(goType, ".[]*{} ") {
		return ""
	}
	return goType
}
