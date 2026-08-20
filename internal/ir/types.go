package ir

import "strings"

// TypesByName indexes type definitions by the Go name they declare.
func TypesByName(types []*TypeDef) map[string]*TypeDef {
	byName := make(map[string]*TypeDef, len(types))
	for _, td := range types {
		if td != nil {
			byName[td.Name] = td
		}
	}
	return byName
}

// NamedType returns goType when it is a bare type name rather than a builtin or a
// composite with no single referent.
func NamedType(goType string) string {
	if goType == "" || goType == "any" || strings.ContainsAny(goType, ".[]*{} ") {
		return ""
	}
	return goType
}

// StructNamed returns the struct goType ultimately denotes, following the aliases
// that may stand between the two. It returns nil for anything that does not end at
// a generated struct.
func StructNamed(byName map[string]*TypeDef, goType string) *TypeDef {
	for range len(byName) + 1 {
		td := byName[NamedType(goType)]
		if td == nil {
			return nil
		}
		switch td.Kind {
		case TypeKindStruct:
			return td
		case TypeKindAlias:
			goType = td.GoType
		default:
			return nil
		}
	}
	return nil
}

// TypeKind represents the kind of Go type to generate.
type TypeKind int

const (
	TypeKindStruct TypeKind = iota
	TypeKindAlias
	TypeKindEnum
	TypeKindUnion
)

// String returns the string representation of a TypeKind.
func (k TypeKind) String() string {
	switch k {
	case TypeKindStruct:
		return "struct"
	case TypeKindAlias:
		return "alias"
	case TypeKindEnum:
		return "enum"
	case TypeKindUnion:
		return "union"
	default:
		return "unknown"
	}
}

// TypeDef represents a Go type to be generated.
type TypeDef struct {
	Name          string            // Go type name (PascalCase)
	Description   string            // GoDoc comment
	Kind          TypeKind          // Struct, Alias, Enum, Union
	GoType        string            // For aliases: the underlying Go type string
	Fields        []*Field          // For structs
	EnumValues    []*EnumVal        // For enums
	EnumGoType    string            // For enums: the underlying Go type (e.g., "string", "int")
	UnionTypes    []*UnionVariant   // For oneOf/anyOf unions
	BaseType      string            // For unions: the type holding the properties every variant shares
	BaseEmbedded  bool              // For unions: whether the variants embed BaseType rather than declaring its fields
	Discriminator *DiscriminatorDef // If polymorphic via discriminator
	IsNullable    bool
}

// Field represents a struct field.
type Field struct {
	Name                string // Go field name (PascalCase)
	JSONName            string // Original JSON property name (for struct tag)
	Type                string // Go type expression (e.g., "string", "*int64", "[]User")
	Description         string
	Required            bool
	IsPointer           bool // Whether to use pointer type (nullable or optional)
	OmitEmpty           bool // Whether to add omitempty to JSON tag
	Embedded            bool // Whether this is an embedded (anonymous) field
	Deprecated          bool
	ReadOnly            bool
	WriteOnly           bool
	PrimaryErrorMessage bool // Property annotated x-ms-primary-error-message
	CatchAll            bool // Synthetic field holding the schema's additionalProperties
}

// EnumVal represents one value in an enum type.
type EnumVal struct {
	Name    string // Go const name (e.g., PetStatusAvailable)
	Literal string // Rendered Go constant literal (quoted for strings)
}

// UnionVariant represents one arm of a oneOf/anyOf union.
type UnionVariant struct {
	TypeName           string // Go type name of this variant
	DiscriminatorValue string // For discriminator-based dispatch
}

// DiscriminatorDef describes how to dispatch a union type.
type DiscriminatorDef struct {
	PropertyName string
	Mapping      map[string]string // discriminator value -> Go type name
}
