package analyzer

import "github.com/parallelworks/openapi-client-generator/internal/ir"

// linkUnionBases records on every union the one type all of its variants embed,
// when there is exactly one. Reading a field the variants share is otherwise a
// type switch over every variant, which the compiler cannot keep exhaustive as
// variants are added.
func linkUnionBases(types []*ir.TypeDef) {
	byName := ir.TypesByName(types)
	for _, td := range types {
		if td == nil || td.Kind != ir.TypeKindUnion || len(td.UnionTypes) == 0 {
			continue
		}
		td.BaseType = sharedEmbeddedType(byName, td.UnionTypes)
	}
}

// sharedEmbeddedType returns the single type every variant embeds by value, or ""
// when the variants share none or share more than one.
func sharedEmbeddedType(byName map[string]*ir.TypeDef, variants []*ir.UnionVariant) string {
	var shared []string
	for i, v := range variants {
		embedded := embeddedTypeNames(byName, v.TypeName)
		if i == 0 {
			shared = embedded
		} else {
			shared = intersection(shared, embedded)
		}
		if len(shared) == 0 {
			return ""
		}
	}
	if len(shared) != 1 {
		return ""
	}
	return shared[0]
}

// embeddedTypeNames returns the struct types goType embeds by value.
func embeddedTypeNames(byName map[string]*ir.TypeDef, goType string) []string {
	td := ir.StructNamed(byName, goType)
	if td == nil {
		return nil
	}
	var names []string
	for _, f := range td.Fields {
		// A cycle-broken embed is already a pointer, and its field name no longer
		// matches the type expression an accessor would have to name.
		if !f.Embedded || f.IsPointer {
			continue
		}
		if ir.StructNamed(byName, f.Type) == nil {
			continue
		}
		names = append(names, f.Name)
	}
	return names
}

func intersection(a, b []string) []string {
	inB := make(map[string]bool, len(b))
	for _, name := range b {
		inB[name] = true
	}
	var both []string
	for _, name := range a {
		if inB[name] {
			both = append(both, name)
		}
	}
	return both
}
