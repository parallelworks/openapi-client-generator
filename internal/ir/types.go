package ir

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
