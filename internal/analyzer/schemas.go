package analyzer

import (
	"fmt"
	"slices"

	highbase "github.com/pb33f/libopenapi/datamodel/high/base"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
	"github.com/parallelworks/openapi-client-generator/internal/naming"
)

// convertSchema converts a single OpenAPI schema into an IR TypeDef.
func (a *Analyzer) convertSchema(goName, specName string, schema *highbase.Schema) (*ir.TypeDef, error) {
	nullable := isNullable(schema)

	// Enum type.
	if len(schema.Enum) > 0 {
		return a.convertEnum(goName, schema, nullable)
	}

	// Composition types: allOf, oneOf, anyOf.
	if len(schema.AllOf) > 0 {
		return a.convertAllOf(goName, schema, nullable)
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
		return a.convertObject(goName, schema, nullable)
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

	td := &ir.TypeDef{
		Name:        goName,
		Description: schema.Description,
		Kind:        ir.TypeKindEnum,
		EnumGoType:  underlyingType,
		IsNullable:  nullable,
	}

	for _, enumNode := range schema.Enum {
		if enumNode == nil {
			continue
		}
		val := enumNode.Value
		if val == "" || val == "null" {
			continue
		}
		constName := goName + naming.ToGoName(val)
		td.EnumValues = append(td.EnumValues, &ir.EnumVal{
			Name:  constName,
			Value: val,
		})
	}

	return td, nil
}

// convertAllOf creates a struct TypeDef from an allOf composition.
// $ref entries become embedded fields; inline schemas have their properties merged.
func (a *Analyzer) convertAllOf(goName string, schema *highbase.Schema, nullable bool) (*ir.TypeDef, error) {
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
			goTypeName := naming.ToGoName(refName)
			if td, ok := a.typesBySchema[refName]; ok {
				goTypeName = td.Name
			}
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

			required := requiredSet[propName]
			propNullable := isNullable(propSchema)
			goType := a.resolveGoType(propSchema)
			isPointer := !required || propNullable

			if isPointer && goType != "any" && !isSliceType(goType) && !isMapType(goType) {
				goType = "*" + goType
			}

			td.Fields = append(td.Fields, &ir.Field{
				Name:      naming.ToGoName(propName),
				JSONName:  propName,
				Type:      goType,
				Required:  required,
				IsPointer: isPointer,
				OmitEmpty: !required,
			})
		}
	}

	return td, nil
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
				refName := refToSchemaName(v)
				goTypeName := naming.ToGoName(refName)
				if existing, ok := a.typesBySchema[refName]; ok {
					goTypeName = existing.Name
				}
				td.Discriminator.Mapping[k] = goTypeName
				discMapping[v] = k
			}
		}
	}

	for _, proxy := range variants {
		ref := proxy.GetReference()
		refName := refToSchemaName(ref)

		var typeName string
		if refName != "" {
			typeName = naming.ToGoName(refName)
			if existing, ok := a.typesBySchema[refName]; ok {
				typeName = existing.Name
			}
		} else {
			// Inline variant: use "any" as the type.
			typeName = "any"
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
func (a *Analyzer) convertObject(goName string, schema *highbase.Schema, nullable bool) (*ir.TypeDef, error) {
	// If no defined properties and additionalProperties is set, generate a map alias.
	hasProperties := schema.Properties != nil && schema.Properties.Len() > 0
	if !hasProperties && schema.AdditionalProperties != nil {
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

		required := requiredSet[propName]
		propNullable := isNullable(propSchema)
		goType := a.resolveGoType(propSchema)
		isPointer := !required || propNullable

		if isPointer && goType != "any" && !isSliceType(goType) && !isMapType(goType) {
			goType = "*" + goType
		}

		td.Fields = append(td.Fields, &ir.Field{
			Name:      naming.ToGoName(propName),
			JSONName:  propName,
			Type:      goType,
			Required:  required,
			IsPointer: isPointer,
			OmitEmpty: !required,
			ReadOnly:  schema.ReadOnly != nil && *schema.ReadOnly,
			WriteOnly: schema.WriteOnly != nil && *schema.WriteOnly,
		})
	}

	// If the object has both properties and additionalProperties, add an extra field.
	if schema.AdditionalProperties != nil {
		mapValueType := a.resolveAdditionalPropertiesType(schema)
		td.Fields = append(td.Fields, &ir.Field{
			Name:      "AdditionalProperties",
			JSONName:  "-",
			Type:      "map[string]" + mapValueType,
			OmitEmpty: true,
		})
	}

	return td, nil
}

// convertAdditionalPropertiesMap creates a map alias when an object has
// additionalProperties but no defined properties.
func (a *Analyzer) convertAdditionalPropertiesMap(goName string, schema *highbase.Schema, nullable bool) (*ir.TypeDef, error) {
	mapValueType := a.resolveAdditionalPropertiesType(schema)
	return &ir.TypeDef{
		Name:        goName,
		Description: schema.Description,
		Kind:        ir.TypeKindAlias,
		GoType:      "map[string]" + mapValueType,
		IsNullable:  nullable,
	}, nil
}

// resolveAdditionalPropertiesType returns the Go value type for additionalProperties.
func (a *Analyzer) resolveAdditionalPropertiesType(schema *highbase.Schema) string {
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
			return a.resolveGoType(apSchema)
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
			elemType = a.resolveGoType(itemSchema)
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
// to known types, arrays, and primitives.
func (a *Analyzer) resolveGoType(schema *highbase.Schema) string {
	// Check if this schema is a $ref pointing to a known component schema.
	if schema.ParentProxy != nil {
		ref := schema.ParentProxy.GetReference()
		if ref != "" {
			refName := refToSchemaName(ref)
			if refName != "" {
				if td, ok := a.typesBySchema[refName]; ok {
					return td.Name
				}
				// Not yet converted, use the Go name directly.
				return naming.ToGoName(refName)
			}
		}
	}

	// Enum type referenced inline -- use the primary type.
	primaryType := primaryType(schema)

	switch primaryType {
	case "object":
		// Inline objects without properties -> map[string]any.
		if schema.Properties == nil || schema.Properties.Len() == 0 {
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
				elemType = a.resolveGoType(itemSchema)
			}
		}
		return "[]" + elemType
	case "string", "integer", "number", "boolean":
		return goTypeForPrimitive(primaryType, schema.Format)
	default:
		return "any"
	}
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
// indicated by type: ["string", "null"]. In 3.0, it's nullable: true.
func isNullable(schema *highbase.Schema) bool {
	if schema.Nullable != nil && *schema.Nullable {
		return true
	}
	return slices.Contains(schema.Type, "null")
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
