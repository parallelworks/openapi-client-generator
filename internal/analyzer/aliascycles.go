package analyzer

import (
	"slices"
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

// breakStructCycles turns into a pointer every struct field that would make its
// type contain itself by value, which Go rejects the same way as a recursive
// alias. A field that is already a pointer, slice, or map stops the recursion on
// its own; a required $ref to the enclosing type does not.
func breakStructCycles(types []*ir.TypeDef) {
	byName := make(map[string]*ir.TypeDef, len(types))
	for _, td := range types {
		if td != nil {
			byName[td.Name] = td
		}
	}

	// containedStruct returns the struct a field of this type holds by value,
	// following aliases to the definition they stand for.
	containedStruct := func(goType string) *ir.TypeDef {
		for range len(types) + 1 {
			td, ok := byName[namedType(goType)]
			if !ok {
				return nil
			}
			switch td.Kind {
			case ir.TypeKindStruct:
				return td
			case ir.TypeKindAlias:
				goType = td.GoType
			default:
				return nil
			}
		}
		return nil
	}

	const (
		visiting = 1
		done     = 2
	)
	state := make(map[string]int, len(byName))

	var walk func(td *ir.TypeDef)
	walk = func(td *ir.TypeDef) {
		state[td.Name] = visiting
		for _, f := range td.Fields {
			next := containedStruct(f.Type)
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

// dropShadowedCatchAlls removes the catch-all from a struct that embeds a type
// which already has one. The generated marshalers shadow the struct to reach
// encoding/json, and a shadow still promotes an embedded type's MarshalJSON — so
// the two catch-alls would fight and the embedded one would win, emitting only
// its own fields. Leaving the outer schema's undeclared properties uncollected
// is the narrower loss, and it is what the generator did before composed schemas
// collected any at all.
//
// Handling both at once needs the shadow replaced with field-by-field marshaling.
func dropShadowedCatchAlls(types []*ir.TypeDef) {
	byName := make(map[string]*ir.TypeDef, len(types))
	for _, td := range types {
		if td != nil && td.Kind == ir.TypeKindStruct {
			byName[td.Name] = td
		}
	}

	hasCatchAll := func(td *ir.TypeDef) bool {
		return slices.ContainsFunc(td.Fields, func(f *ir.Field) bool { return f.CatchAll })
	}

	// Reports whether td or anything it embeds carries a catch-all.
	var embedsCatchAll func(td *ir.TypeDef, depth int) bool
	embedsCatchAll = func(td *ir.TypeDef, depth int) bool {
		if td == nil || depth > len(types) {
			return false
		}
		for _, f := range td.Fields {
			if !f.Embedded {
				continue
			}
			embedded := byName[namedType(strings.TrimPrefix(f.Type, "*"))]
			if embedded == nil {
				continue
			}
			if hasCatchAll(embedded) || embedsCatchAll(embedded, depth+1) {
				return true
			}
		}
		return false
	}

	for _, td := range types {
		if td == nil || td.Kind != ir.TypeKindStruct || !hasCatchAll(td) {
			continue
		}
		if embedsCatchAll(td, 0) {
			td.Fields = slices.DeleteFunc(td.Fields, func(f *ir.Field) bool { return f.CatchAll })
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
