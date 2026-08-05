package analyzer

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	highbase "github.com/pb33f/libopenapi/datamodel/high/base"

	naming "github.com/giraffesyo/openapi-go-naming"
	"github.com/parallelworks/openapi-client-generator/internal/ir"
)

// convertSchema converts a single OpenAPI schema into an IR TypeDef.
func (a *Analyzer) convertSchema(goName, specName string, schema *highbase.Schema) (*ir.TypeDef, error) {
	nullable := isNullable(schema)

	// Enum type.
	if len(schema.Enum) > 0 {
		return a.convertEnum(goName, schema, nullable)
	}

	// A oneOf/anyOf whose only other member is `type: null` is how OpenAPI 3.1
	// spells "nullable T"; it offers no choice to model, so generate T itself.
	if variant, ok := nullableUnionVariant(schema); ok {
		return a.convertNullableUnion(goName, specName, schema, variant)
	}
	if goType, ok := a.uniformUnionGoType(schema, goName); ok {
		return &ir.TypeDef{
			Name:        goName,
			Description: schema.Description,
			Kind:        ir.TypeKindAlias,
			GoType:      goType,
			IsNullable:  nullable,
		}, nil
	}

	// Composition types: allOf, oneOf, anyOf.
	if len(schema.AllOf) > 0 {
		return a.convertAllOf(goName, schema, nullable, a.multipartBodies[specName])
	}
	if len(schema.OneOf) > 0 {
		return a.convertOneOf(goName, schema, nullable)
	}
	if len(schema.AnyOf) > 0 {
		return a.convertAnyOf(goName, schema, nullable)
	}

	primaryType := primaryType(schema)

	switch primaryType {
	case "object":
		return a.convertObject(goName, schema, nullable, a.multipartBodies[specName])
	case "array":
		return a.convertArray(goName, schema, nullable)
	case "string", "integer", "number", "boolean":
		return a.convertPrimitive(goName, primaryType, schema, nullable)
	default:
		// Unknown or missing type -- create an alias to any.
		return &ir.TypeDef{
			Name:        goName,
			Description: schema.Description,
			Kind:        ir.TypeKindAlias,
			GoType:      "any",
			IsNullable:  nullable,
		}, nil
	}
}

// convertEnum creates an enum TypeDef.
func (a *Analyzer) convertEnum(goName string, schema *highbase.Schema, nullable bool) (*ir.TypeDef, error) {
	underlyingType := goTypeForPrimitive(primaryType(schema), schema.Format)

	// An enum over a type Go can't declare constants of (time.Time, []byte, any)
	// carries no useful named values, so emit a plain alias with no const block.
	if !constableType(underlyingType) {
		return &ir.TypeDef{
			Name:        goName,
			Description: schema.Description,
			Kind:        ir.TypeKindAlias,
			GoType:      underlyingType,
			IsNullable:  nullable,
		}, nil
	}

	td := &ir.TypeDef{
		Name:        goName,
		Description: schema.Description,
		Kind:        ir.TypeKindEnum,
		EnumGoType:  underlyingType,
		IsNullable:  nullable,
	}

	for _, enumNode := range schema.Enum {
		if enumNode == nil || enumNode.Tag == "!!null" {
			continue
		}
		raw := enumNode.Value
		// Skip a value that has no valid Go constant literal for this type (an
		// out-of-range, NaN/Inf, or non-numeric member of a numeric enum); the
		// generated code must always compile.
		literal, ok := enumConstLiteral(underlyingType, raw)
		if !ok {
			continue
		}
		// Unique keeps the const unique against package types/other consts —
		// two values that sanitize to the same identifier ("a-b"/"a b"), or a const
		// that matches a schema-named type, would otherwise fail to compile.
		constName := a.namer.Unique(enumConstName(goName, raw))
		td.EnumValues = append(td.EnumValues, &ir.EnumVal{
			Name:    constName,
			Literal: literal,
		})
	}

	return td, nil
}

// constableType reports whether Go can declare a typed constant of goType (the
// const-able primitives goTypeForPrimitive produces; time.Time/[]byte/any are not).
func constableType(goType string) bool {
	switch goType {
	case "string", "bool", "int32", "int64", "float32", "float64":
		return true
	}
	return false
}

// enumConstLiteral renders a raw enum scalar (always a YAML string) as a Go
// constant literal for goType, returning ok=false when the value can't be
// represented (out of range, NaN/Inf, or unparseable), so the caller skips it.
func enumConstLiteral(goType, raw string) (string, bool) {
	switch goType {
	case "string":
		return strconv.Quote(raw), true
	case "bool":
		if b, err := strconv.ParseBool(raw); err == nil {
			return strconv.FormatBool(b), true
		}
	case "int32", "int64":
		bits := 64
		if goType == "int32" {
			bits = 32
		}
		// Base 10 first so a leading-zero decimal (010) stays decimal; fall back to
		// base 0 only on a syntax error (hex/octal/underscored like 0x1F/0o17),
		// never on ErrRange — an overflowing value must be dropped, not reread as
		// octal.
		n, err := strconv.ParseInt(raw, 10, bits)
		if errors.Is(err, strconv.ErrSyntax) {
			n, err = strconv.ParseInt(raw, 0, bits)
		}
		if err == nil {
			return strconv.FormatInt(n, 10), true
		}
	case "float32", "float64":
		bits := 64
		if goType == "float32" {
			bits = 32
		}
		if f, err := strconv.ParseFloat(raw, bits); err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) {
			return strconv.FormatFloat(f, 'f', -1, bits), true
		}
	}
	return "", false
}

// convertAllOf creates a struct TypeDef from an allOf composition.
// $ref entries become embedded fields; inline schemas have their properties merged.
func (a *Analyzer) convertAllOf(goName string, schema *highbase.Schema, nullable, multipartBody bool) (*ir.TypeDef, error) {
	td := &ir.TypeDef{
		Name:        goName,
		Description: schema.Description,
		Kind:        ir.TypeKindStruct,
		IsNullable:  nullable,
	}

	// Collect required fields from the parent schema.
	requiredSet := make(map[string]bool, len(schema.Required))
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	for _, proxy := range schema.AllOf {
		ref := proxy.GetReference()
		refName := refToSchemaName(ref)

		if refName != "" {
			// $ref to a known component schema: add as embedded field.
			goTypeName := a.goTypeForSchemaName(refName)
			td.Fields = append(td.Fields, &ir.Field{
				Name:     goTypeName,
				Type:     goTypeName,
				Embedded: true,
			})
			continue
		}

		// Inline schema: merge its properties into this struct.
		entrySchema, err := proxy.BuildSchema()
		if err != nil {
			return nil, fmt.Errorf("building allOf entry schema: %w", err)
		}
		if entrySchema == nil {
			continue
		}

		// Merge required from the allOf entry.
		for _, r := range entrySchema.Required {
			requiredSet[r] = true
		}

		if entrySchema.Properties == nil {
			continue
		}

		for propName, propProxy := range entrySchema.Properties.FromOldest() {
			propSchema, err := propProxy.BuildSchema()
			if err != nil {
				return nil, fmt.Errorf("building allOf property %q schema: %w", propName, err)
			}
			if propSchema == nil {
				continue
			}

			td.Fields = append(td.Fields, a.convertProperty(goName, propName, propSchema, requiredSet[propName], multipartBody))
		}
	}

	return td, nil
}

// convertProperty converts one object property into a struct field. multipartBody
// marks a schema sent as multipart form data, whose binary properties are file
// parts rather than byte slices.
func (a *Analyzer) convertProperty(goName, propName string, propSchema *highbase.Schema, required, multipartBody bool) *ir.Field {
	goType := a.resolveGoType(propSchema, goName+naming.Exported(propName))
	if multipartBody {
		if fileType, ok := formFileType(propSchema); ok {
			goType = fileType
		}
	}

	isPointer := !required || isNullable(propSchema)
	if isPointer && goType != "any" && !isSliceType(goType) && !isMapType(goType) {
		goType = "*" + goType
	}

	return &ir.Field{
		Name:                naming.Exported(propName),
		JSONName:            propName,
		Type:                goType,
		Description:         propSchema.Description,
		Required:            required,
		IsPointer:           isPointer,
		OmitEmpty:           !required,
		Deprecated:          propSchema.Deprecated != nil && *propSchema.Deprecated,
		ReadOnly:            propSchema.ReadOnly != nil && *propSchema.ReadOnly,
		WriteOnly:           propSchema.WriteOnly != nil && *propSchema.WriteOnly,
		PrimaryErrorMessage: isPrimaryErrorMessage(propSchema),
	}
}

// isPrimaryErrorMessage reports whether a property schema carries Kiota's
// x-ms-primary-error-message extension marking it as the human-readable
// error message.
func isPrimaryErrorMessage(schema *highbase.Schema) bool {
	if schema.Extensions == nil {
		return false
	}
	for name, node := range schema.Extensions.FromOldest() {
		if name != "x-ms-primary-error-message" || node == nil {
			continue
		}
		var enabled bool
		if err := node.Decode(&enabled); err != nil {
			return false
		}
		return enabled
	}
	return false
}

// convertOneOf creates a union TypeDef from a oneOf composition.
func (a *Analyzer) convertOneOf(goName string, schema *highbase.Schema, nullable bool) (*ir.TypeDef, error) {
	return a.convertUnion(goName, schema, schema.OneOf, nullable)
}

// convertAnyOf creates a union TypeDef from an anyOf composition.
func (a *Analyzer) convertAnyOf(goName string, schema *highbase.Schema, nullable bool) (*ir.TypeDef, error) {
	return a.convertUnion(goName, schema, schema.AnyOf, nullable)
}

// convertUnion creates a TypeKindUnion TypeDef from oneOf or anyOf variants.
func (a *Analyzer) convertUnion(goName string, schema *highbase.Schema, variants []*highbase.SchemaProxy, nullable bool) (*ir.TypeDef, error) {
	td := &ir.TypeDef{
		Name:        goName,
		Description: schema.Description,
		Kind:        ir.TypeKindUnion,
		IsNullable:  nullable,
	}

	// Build discriminator mapping if present.
	var discMapping map[string]string
	if schema.Discriminator != nil {
		td.Discriminator = &ir.DiscriminatorDef{
			PropertyName: schema.Discriminator.PropertyName,
			Mapping:      make(map[string]string),
		}
		if schema.Discriminator.Mapping != nil {
			discMapping = make(map[string]string)
			for k, v := range schema.Discriminator.Mapping.FromOldest() {
				// v is a $ref like "#/components/schemas/Circle"
				td.Discriminator.Mapping[k] = a.goTypeForSchemaName(refToSchemaName(v))
				discMapping[v] = k
			}
		}
		// Without an explicit mapping the spec says the discriminator value is the
		// variant's schema name; deriving it keeps the decoder from treating every
		// payload as an unknown variant.
		if len(td.Discriminator.Mapping) == 0 {
			discMapping = make(map[string]string)
			for _, proxy := range variants {
				ref := proxy.GetReference()
				refName := refToSchemaName(ref)
				if refName == "" {
					continue
				}
				td.Discriminator.Mapping[refName] = a.goTypeForSchemaName(refName)
				discMapping[ref] = refName
			}
		}
	}

	for _, proxy := range variants {
		// A `type: null` member says the union is nullable; it is not one of the
		// shapes the value can take, so it gets no variant of its own.
		if isNullVariant(proxy) {
			continue
		}

		ref := proxy.GetReference()
		refName := refToSchemaName(ref)

		typeName := "any"
		if refName != "" {
			typeName = a.goTypeForSchemaName(refName)
		} else if variantSchema, err := proxy.BuildSchema(); err == nil && variantSchema != nil {
			// An inline variant still has a Go type; without one it would decode
			// into nothing and the payloads it covers would fail to unmarshal.
			typeName = a.resolveGoType(variantSchema, suffixHint(goName, "Variant"))
		}

		variant := &ir.UnionVariant{
			TypeName: typeName,
		}

		// Associate discriminator value if available.
		if discMapping != nil && ref != "" {
			variant.DiscriminatorValue = discMapping[ref]
		}

		td.UnionTypes = append(td.UnionTypes, variant)
	}

	return td, nil
}

// convertObject creates a struct TypeDef from an object schema.
func (a *Analyzer) convertObject(goName string, schema *highbase.Schema, nullable, multipartBody bool) (*ir.TypeDef, error) {
	// If no defined properties and additionalProperties is set, generate a map alias.
	hasProperties := schema.Properties != nil && schema.Properties.Len() > 0
	if !hasProperties && allowsAdditionalProperties(schema) {
		return a.convertAdditionalPropertiesMap(goName, schema, nullable)
	}

	td := &ir.TypeDef{
		Name:        goName,
		Description: schema.Description,
		Kind:        ir.TypeKindStruct,
		IsNullable:  nullable,
	}

	if schema.Properties == nil {
		return td, nil
	}

	requiredSet := make(map[string]bool, len(schema.Required))
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	for propName, propProxy := range schema.Properties.FromOldest() {
		propSchema, err := propProxy.BuildSchema()
		if err != nil {
			return nil, fmt.Errorf("building property %q schema: %w", propName, err)
		}
		if propSchema == nil {
			continue
		}

		td.Fields = append(td.Fields, a.convertProperty(goName, propName, propSchema, requiredSet[propName], multipartBody))
	}

	// If the object has both properties and additionalProperties, add an extra field.
	// The generator gives such structs MarshalJSON/UnmarshalJSON so the map is
	// inlined into the object rather than nested under a key of its own.
	if allowsAdditionalProperties(schema) {
		mapValueType := a.resolveAdditionalPropertiesType(schema, goName)
		td.Fields = append(td.Fields, &ir.Field{
			Name:        catchAllFieldName(td.Fields),
			JSONName:    "-",
			Type:        "map[string]" + mapValueType,
			Description: "Properties not defined by the schema.",
			CatchAll:    true,
		})
	}

	return td, nil
}

// allowsAdditionalProperties reports whether the schema permits undeclared
// properties. `additionalProperties: false` forbids them, so no catch-all is
// generated for it.
func allowsAdditionalProperties(schema *highbase.Schema) bool {
	ap := schema.AdditionalProperties
	if ap == nil {
		return false
	}
	return !ap.IsB() || ap.B
}

// catchAllFieldName picks a Go name for the synthetic additionalProperties field
// that no declared property has already taken.
func catchAllFieldName(fields []*ir.Field) string {
	name := "AdditionalProperties"
	taken := func(candidate string) bool {
		return slices.ContainsFunc(fields, func(f *ir.Field) bool { return f.Name == candidate })
	}
	for i := 2; taken(name); i++ {
		name = "AdditionalProperties" + strconv.Itoa(i)
	}
	return name
}

// formFileType returns the Go type for a multipart property carrying file content.
func formFileType(schema *highbase.Schema) (string, bool) {
	if variant, ok := nullableUnionVariant(schema); ok {
		return formFileType(variant)
	}
	if isBinarySchema(schema) {
		return "FormFile", true
	}
	if primaryType(schema) == "array" && schema.Items != nil && schema.Items.IsA() {
		items, err := schema.Items.A.BuildSchema()
		if err == nil && items != nil && isBinarySchema(items) {
			return "[]FormFile", true
		}
	}
	return "", false
}

// isBinarySchema reports whether a schema is `type: string, format: binary`,
// which inside a multipart body means file content rather than text.
func isBinarySchema(schema *highbase.Schema) bool {
	return primaryType(schema) == "string" && schema.Format == "binary"
}

// convertAdditionalPropertiesMap creates a map alias when an object has
// additionalProperties but no defined properties.
func (a *Analyzer) convertAdditionalPropertiesMap(goName string, schema *highbase.Schema, nullable bool) (*ir.TypeDef, error) {
	mapValueType := a.resolveAdditionalPropertiesType(schema, goName)
	return &ir.TypeDef{
		Name:        goName,
		Description: schema.Description,
		Kind:        ir.TypeKindAlias,
		GoType:      "map[string]" + mapValueType,
		IsNullable:  nullable,
	}, nil
}

// resolveAdditionalPropertiesType returns the Go value type for additionalProperties.
func (a *Analyzer) resolveAdditionalPropertiesType(schema *highbase.Schema, nameHint string) string {
	ap := schema.AdditionalProperties
	if ap == nil {
		return "any"
	}
	// Bool true means any value type.
	if ap.IsB() {
		return "any"
	}
	// Schema-typed additionalProperties.
	if ap.IsA() && ap.A != nil {
		apSchema, err := ap.A.BuildSchema()
		if err == nil && apSchema != nil {
			return a.resolveGoType(apSchema, suffixHint(nameHint, "Value"))
		}
	}
	return "any"
}

// convertArray creates an alias TypeDef for an array schema.
func (a *Analyzer) convertArray(goName string, schema *highbase.Schema, nullable bool) (*ir.TypeDef, error) {
	elemType := "any"
	if schema.Items != nil && schema.Items.IsA() {
		itemSchema, err := schema.Items.A.BuildSchema()
		if err != nil {
			return nil, fmt.Errorf("building array items schema: %w", err)
		}
		if itemSchema != nil {
			elemType = a.resolveGoType(itemSchema, suffixHint(goName, "Item"))
		}
	}

	return &ir.TypeDef{
		Name:        goName,
		Description: schema.Description,
		Kind:        ir.TypeKindAlias,
		GoType:      "[]" + elemType,
		IsNullable:  nullable,
	}, nil
}

// convertPrimitive creates an alias TypeDef for a primitive schema.
func (a *Analyzer) convertPrimitive(goName, primaryType string, schema *highbase.Schema, nullable bool) (*ir.TypeDef, error) {
	goType := goTypeForPrimitive(primaryType, schema.Format)

	return &ir.TypeDef{
		Name:        goName,
		Description: schema.Description,
		Kind:        ir.TypeKindAlias,
		GoType:      goType,
		IsNullable:  nullable,
	}, nil
}

// resolveGoType determines the Go type expression for a schema, handling refs
// to known types, arrays, primitives, and inline unions. nameHint carries the
// naming context (parent type + field) used when a type must be synthesized;
// pass "" when no context is available.
func (a *Analyzer) resolveGoType(schema *highbase.Schema, nameHint string) string {
	// Check if this schema is a $ref pointing to a known component schema.
	if goType := a.refGoType(schema); goType != "" {
		return goType
	}

	// "nullable T" spelled as a union resolves to T; the pointer that carries the
	// null comes from the field or parameter being optional/nullable.
	if variant, ok := nullableUnionVariant(schema); ok {
		return a.resolveGoType(variant, nameHint)
	}
	if goType, ok := a.uniformUnionGoType(schema, nameHint); ok {
		return goType
	}

	// Inline oneOf/anyOf: synthesize a named union so $ref variants stay typed.
	if name, ok := a.synthesizeInlineUnion(schema, nameHint); ok {
		return name
	}

	// Enum type referenced inline -- use the primary type.
	primaryType := primaryType(schema)

	switch primaryType {
	case "object":
		// Inline objects without properties -> a map of the additionalProperties type.
		if schema.Properties == nil || schema.Properties.Len() == 0 {
			if allowsAdditionalProperties(schema) {
				return "map[string]" + a.resolveAdditionalPropertiesType(schema, nameHint)
			}
			return "map[string]any"
		}
		// Complex inline object -- for now use any. Full support would create
		// an anonymous struct or a named type.
		return "any"
	case "array":
		elemType := "any"
		if schema.Items != nil && schema.Items.IsA() {
			itemSchema, _ := schema.Items.A.BuildSchema()
			if itemSchema != nil {
				elemType = a.resolveGoType(itemSchema, suffixHint(nameHint, "Item"))
			}
		}
		return "[]" + elemType
	case "string", "integer", "number", "boolean":
		return goTypeForPrimitive(primaryType, schema.Format)
	default:
		return "any"
	}
}

// synthesizeInlineUnion creates a named union TypeDef for an inline
// oneOf/anyOf schema so its $ref variants keep their generated types instead
// of degrading to any. Identical unions (same variants and discriminator) are
// synthesized once and reuse the first occurrence's name; the resulting types
// are emitted after the component schemas. Returns false when the schema is
// not a union, has no $ref variants worth naming, or no nameHint is available.
func (a *Analyzer) synthesizeInlineUnion(schema *highbase.Schema, nameHint string) (string, bool) {
	variants := schema.OneOf
	kind := "oneOf"
	if len(variants) == 0 {
		variants = schema.AnyOf
		kind = "anyOf"
	}
	// A titled union names itself, which keeps the generated name stable when
	// the operation that reaches it first changes. Untitled unions fall back to
	// the caller's hint.
	if schema.Title != "" {
		nameHint = schema.Title
	}
	if len(variants) == 0 || nameHint == "" {
		return "", false
	}

	// Key on what each member resolves to rather than on its $ref, so two inline
	// unions of different shapes don't collapse onto one synthesized type.
	members := make([]string, 0, len(variants))
	typed := false
	for _, proxy := range variants {
		if isNullVariant(proxy) {
			continue
		}
		if ref := proxy.GetReference(); ref != "" {
			members = append(members, ref)
			typed = true
			continue
		}
		variantSchema, err := proxy.BuildSchema()
		if err != nil || variantSchema == nil {
			return "", false
		}
		goType := a.resolveGoType(variantSchema, suffixHint(nameHint, "Variant"))
		members = append(members, goType)
		typed = typed || goType != "any"
	}
	// A union whose members all decode into any is just any; naming it would add a
	// type that carries no more information than the bare interface.
	if !typed || len(members) < 2 {
		return "", false
	}

	key := kind + "|" + strings.Join(members, ",")
	if schema.Discriminator != nil {
		key += "|" + schema.Discriminator.PropertyName
	}
	if existing, ok := a.synthesizedByKey[key]; ok {
		return existing.Name, true
	}

	goName := a.namer.Unique(naming.Exported(nameHint))
	td, err := a.convertUnion(goName, schema, variants, isNullable(schema))
	if err != nil {
		return "", false
	}
	a.synthesizedByKey[key] = td
	a.synthesized = append(a.synthesized, td)
	return goName, true
}

// suffixHint appends a suffix to a naming hint, preserving emptiness so that
// hintless contexts stay hintless.
func suffixHint(nameHint, suffix string) string {
	if nameHint == "" {
		return ""
	}
	return nameHint + suffix
}

// primaryType extracts the primary (non-null) type from the schema's Type array.
// In OpenAPI 3.1, type can be ["string", "null"] to indicate nullable.
func primaryType(schema *highbase.Schema) string {
	for _, t := range schema.Type {
		if t != "null" {
			return t
		}
	}
	return ""
}

// isNullable checks whether a schema is nullable. In OpenAPI 3.1, this is
// indicated by type: ["string", "null"] or a {"type": "null"} member of a
// oneOf/anyOf. In 3.0, it's nullable: true.
func isNullable(schema *highbase.Schema) bool {
	if schema.Nullable != nil && *schema.Nullable {
		return true
	}
	if slices.Contains(schema.Type, "null") {
		return true
	}
	return slices.ContainsFunc(unionVariants(schema), isNullVariant)
}

// unionVariants returns a schema's oneOf variants, or its anyOf variants when it
// has no oneOf.
func unionVariants(schema *highbase.Schema) []*highbase.SchemaProxy {
	if len(schema.OneOf) > 0 {
		return schema.OneOf
	}
	return schema.AnyOf
}

// isNullVariant reports whether a union member is the bare {"type": "null"}
// schema that makes the union nullable.
func isNullVariant(proxy *highbase.SchemaProxy) bool {
	schema, err := proxy.BuildSchema()
	if err != nil || schema == nil {
		return false
	}
	return primaryType(schema) == "" && slices.Contains(schema.Type, "null")
}

// nullableUnionVariant returns the sole non-null member of a oneOf/anyOf — the
// OpenAPI 3.1 spelling of "nullable T"; a real choice returns ok=false.
func nullableUnionVariant(schema *highbase.Schema) (*highbase.Schema, bool) {
	variants := unionVariants(schema)
	if len(variants) == 0 {
		return nil, false
	}

	var only *highbase.Schema
	for _, proxy := range variants {
		if isNullVariant(proxy) {
			continue
		}
		if only != nil {
			return nil, false
		}
		variant, err := proxy.BuildSchema()
		if err != nil || variant == nil {
			return nil, false
		}
		only = variant
	}
	if only == nil {
		return nil, false
	}
	return only, true
}

// uniformUnionGoType returns the Go type of a oneOf/anyOf whose non-null members
// all resolve to it — several refinements of one type are that type, not a choice.
func (a *Analyzer) uniformUnionGoType(schema *highbase.Schema, nameHint string) (string, bool) {
	variants := unionVariants(schema)
	if len(variants) < 2 {
		return "", false
	}

	// Resolving a member can synthesize a type for it, so use the same hint the
	// union path would: probing must not let a member claim the name the union
	// itself will need when the members turn out to disagree.
	memberHint := suffixHint(nameHint, "Variant")

	goType := ""
	for _, proxy := range variants {
		if isNullVariant(proxy) {
			continue
		}
		variant, err := proxy.BuildSchema()
		if err != nil || variant == nil {
			return "", false
		}
		resolved := a.resolveGoType(variant, memberHint)
		if resolved == "any" || (goType != "" && resolved != goType) {
			return "", false
		}
		goType = resolved
	}
	return goType, goType != ""
}

// convertNullableUnion converts the collapsed "nullable T" union at goName; a
// $ref variant aliases the type it points at.
func (a *Analyzer) convertNullableUnion(goName, specName string, schema, variant *highbase.Schema) (*ir.TypeDef, error) {
	nullable := isNullable(schema)
	if goType := a.refGoType(variant); goType != "" {
		return &ir.TypeDef{
			Name:        goName,
			Description: schema.Description,
			Kind:        ir.TypeKindAlias,
			GoType:      goType,
			IsNullable:  nullable,
		}, nil
	}

	td, err := a.convertSchema(goName, specName, variant)
	if err != nil {
		return nil, err
	}
	td.IsNullable = nullable
	if td.Description == "" {
		td.Description = schema.Description
	}
	return td, nil
}

// refGoType returns the Go type name a $ref schema resolves to, or "" when the
// schema is not a reference to a component schema.
func (a *Analyzer) refGoType(schema *highbase.Schema) string {
	if schema.ParentProxy == nil {
		return ""
	}
	return a.goTypeForRef(schema.ParentProxy.GetReference())
}

// goTypeForRef returns the Go type name a "#/components/schemas/..." reference
// resolves to, or "" when it points elsewhere.
func (a *Analyzer) goTypeForRef(ref string) string {
	refName := refToSchemaName(ref)
	if refName == "" {
		return ""
	}
	return a.goTypeForSchemaName(refName)
}

// goTypeForSchemaName returns the Go type name of a component schema, falling
// back to its exported spelling when it has not been converted yet.
func (a *Analyzer) goTypeForSchemaName(refName string) string {
	if td, ok := a.typesBySchema[refName]; ok {
		return td.Name
	}
	return naming.Exported(refName)
}

// goTypeForPrimitive maps an OpenAPI type + format to a Go type.
func goTypeForPrimitive(typeName, format string) string {
	switch typeName {
	case "string":
		switch format {
		case "date-time":
			return "time.Time"
		case "date":
			return "string"
		case "byte":
			return "[]byte"
		case "binary":
			return "[]byte"
		default:
			return "string"
		}
	case "integer":
		switch format {
		case "int32":
			return "int32"
		case "int64":
			return "int64"
		default:
			return "int64"
		}
	case "number":
		switch format {
		case "float":
			return "float32"
		case "double":
			return "float64"
		default:
			return "float64"
		}
	case "boolean":
		return "bool"
	default:
		return "any"
	}
}

// refToSchemaName extracts the schema name from a $ref string like
// "#/components/schemas/Pet".
func refToSchemaName(ref string) string {
	const prefix = "#/components/schemas/"
	if len(ref) > len(prefix) && ref[:len(prefix)] == prefix {
		return ref[len(prefix):]
	}
	return ""
}

// isSliceType returns true if the Go type string represents a slice.
func isSliceType(goType string) bool {
	return len(goType) >= 2 && goType[:2] == "[]"
}

// isMapType returns true if the Go type string represents a map.
func isMapType(goType string) bool {
	return len(goType) >= 4 && goType[:4] == "map["
}
