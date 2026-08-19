package analyzer

import (
	naming "github.com/giraffesyo/openapi-go-naming"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
)

// linkUnionBases gives every union the type holding the properties its variants
// all carry, when there is one. Reading such a property is otherwise a type
// switch over every variant, which the compiler cannot keep exhaustive as
// variants are added.
//
// A variant set that composes a shared schema through allOf names it directly.
// One that inlines the same properties instead, as generators that flatten
// composition emit, gets a base synthesized from the properties they share.
func (a *Analyzer) linkUnionBases(pkg *ir.Package) {
	byName := ir.TypesByName(pkg.Types)
	for _, td := range pkg.Types {
		if td == nil || td.Kind != ir.TypeKindUnion || len(td.UnionTypes) == 0 {
			continue
		}
		if base := sharedEmbeddedType(byName, td.UnionTypes); base != "" {
			td.BaseType = base
			td.BaseEmbedded = true
			continue
		}
		fields := sharedFields(byName, td.UnionTypes, td.Discriminator)
		if len(fields) == 0 {
			continue
		}
		base := &ir.TypeDef{
			Name:        a.namer.Unique(naming.Exported(td.Name + "Base")),
			Description: "The properties every variant of " + td.Name + " declares.",
			Kind:        ir.TypeKindStruct,
			Fields:      fields,
		}
		pkg.Types = append(pkg.Types, base)
		td.BaseType = base.Name
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

// sharedFields returns copies of the fields every variant declares identically,
// in the first variant's order. It returns nil unless the union has at least two
// distinct struct variants: one variant shares everything with itself, which
// would make a base that only restates it.
func sharedFields(byName map[string]*ir.TypeDef, variants []*ir.UnionVariant, disc *ir.DiscriminatorDef) []*ir.Field {
	var structs []*ir.TypeDef
	seen := make(map[string]bool, len(variants))
	for _, v := range variants {
		if seen[v.TypeName] {
			continue
		}
		seen[v.TypeName] = true
		td := ir.StructNamed(byName, v.TypeName)
		if td == nil {
			return nil
		}
		structs = append(structs, td)
	}
	if len(structs) < 2 {
		return nil
	}

	var shared []*ir.Field
	for _, f := range structs[0].Fields {
		if shareable(f, disc) {
			shared = append(shared, f)
		}
	}
	for _, td := range structs[1:] {
		declared := make(map[string]bool, len(td.Fields))
		for _, f := range td.Fields {
			if shareable(f, disc) {
				declared[fieldKey(f)] = true
			}
		}
		kept := shared[:0]
		for _, f := range shared {
			if declared[fieldKey(f)] {
				kept = append(kept, f)
			}
		}
		shared = kept
		if len(shared) == 0 {
			return nil
		}
	}

	copies := make([]*ir.Field, len(shared))
	for i, f := range shared {
		field := *f
		copies[i] = &field
	}
	return copies
}

// shareable reports whether a field can belong to a synthesized base: an embedded
// type is the other path's business, a catch-all holds what its own schema left
// undeclared rather than a property the variants agree on, and the discriminator
// is how the variants differ, which is also why a spec that spells the base out
// keeps it out of the shared schema.
func shareable(f *ir.Field, disc *ir.DiscriminatorDef) bool {
	if f.Embedded || f.CatchAll {
		return false
	}
	return disc == nil || f.JSONName != disc.PropertyName
}

// fieldKey identifies a field by everything that shapes the Go it generates, so
// two variants agree on a property only when they declare it the same way.
func fieldKey(f *ir.Field) string {
	key := f.Name + "\x00" + f.JSONName + "\x00" + f.Type
	for _, flag := range []bool{f.Required, f.IsPointer, f.OmitEmpty, f.ReadOnly, f.WriteOnly, f.Deprecated} {
		if flag {
			key += "1"
		} else {
			key += "0"
		}
	}
	return key
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
