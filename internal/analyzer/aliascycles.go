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
			// Only the cyclic referent has to go; the slice or map around it is
			// still what the caller gets.
			td.GoType = strings.TrimSuffix(td.GoType, target) + "any"
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

// breakStructCycles turns into a pointer every struct field that would make its
// type contain itself by value, which Go rejects the same way as a recursive
// alias. A field that is already a pointer, slice, or map stops the recursion on
// its own; a required $ref to the enclosing type does not.
func breakStructCycles(types []*ir.TypeDef) {
	byName := ir.TypesByName(types)

	const (
		visiting = 1
		done     = 2
	)
	state := make(map[string]int, len(byName))

	var walk func(td *ir.TypeDef)
	walk = func(td *ir.TypeDef) {
		state[td.Name] = visiting
		for _, f := range td.Fields {
			// A field already written as a pointer, slice, or map stops the
			// recursion on its own, and ir.StructNamed rejects all three.
			next := ir.StructNamed(byName, f.Type)
			if next == nil {
				continue
			}
			if state[next.Name] == visiting {
				// This field closes the cycle, so it is the one to indirect.
				f.Type = "*" + f.Type
				f.IsPointer = true
				continue
			}
			if state[next.Name] != done {
				walk(next)
			}
		}
		state[td.Name] = done
	}

	for _, td := range types {
		if td != nil && td.Kind == ir.TypeKindStruct && state[td.Name] == 0 {
			walk(td)
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
			return ir.NamedType(goType)
		}
	}
}
