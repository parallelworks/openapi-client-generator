package analyzer

import (
	"path/filepath"
	"runtime"
	"testing"

	highbase "github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
	"github.com/parallelworks/openapi-client-generator/internal/parser"
)

func projectRoot() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..")
}

func TestAnalyzePetstore(t *testing.T) {
	specPath := filepath.Join(projectRoot(), "testdata", "petstore.yaml")

	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	a := New(result.Model)
	pkg, err := a.Analyze("petstore")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// Verify package metadata.
	if pkg.Name != "petstore" {
		t.Errorf("pkg.Name = %q, want %q", pkg.Name, "petstore")
	}
	if pkg.Info == nil {
		t.Fatal("expected pkg.Info to be set")
	}
	if pkg.Info.Title != "Petstore" {
		t.Errorf("pkg.Info.Title = %q, want %q", pkg.Info.Title, "Petstore")
	}
	if pkg.Info.Version != "1.0.0" {
		t.Errorf("pkg.Info.Version = %q, want %q", pkg.Info.Version, "1.0.0")
	}
	if len(pkg.Servers) != 1 || pkg.Servers[0].URL != "https://petstore.example.com/v1" {
		t.Errorf("pkg.Servers = %v, want one server at https://petstore.example.com/v1", pkg.Servers)
	}

	if len(pkg.Types) == 0 {
		t.Fatal("expected at least one type")
	}

	typeMap := make(map[string]*ir.TypeDef)
	for _, td := range pkg.Types {
		typeMap[td.Name] = td
		t.Logf("Type: %s (kind=%d)", td.Name, td.Kind)
	}

	// Pet should be a struct.
	pet, ok := typeMap["Pet"]
	if !ok {
		t.Fatal("expected Pet type")
	}
	if pet.Kind != ir.TypeKindStruct {
		t.Errorf("Pet: expected struct, got kind %d", pet.Kind)
	}
	if len(pet.Fields) == 0 {
		t.Error("Pet: expected fields")
	}

	// Check that Pet has id, name fields.
	fieldMap := make(map[string]*ir.Field)
	for _, f := range pet.Fields {
		fieldMap[f.JSONName] = f
		t.Logf("  Pet.%s: type=%s required=%v pointer=%v", f.Name, f.Type, f.Required, f.IsPointer)
	}

	if idField, ok := fieldMap["id"]; !ok {
		t.Error("Pet: missing id field")
	} else {
		if idField.Type != "int64" {
			t.Errorf("Pet.id: expected type int64, got %s", idField.Type)
		}
		if !idField.Required {
			t.Error("Pet.id: expected required")
		}
	}

	if nameField, ok := fieldMap["name"]; !ok {
		t.Error("Pet: missing name field")
	} else {
		if nameField.Type != "string" {
			t.Errorf("Pet.name: expected type string, got %s", nameField.Type)
		}
	}

	// Verify type-level description.
	if pet.Description != "A pet in the store" {
		t.Errorf("Pet.Description = %q, want %q", pet.Description, "A pet in the store")
	}

	// Verify field-level descriptions are extracted from property schemas.
	if idField, ok := fieldMap["id"]; ok {
		if idField.Description != "The unique identifier for the pet" {
			t.Errorf("Pet.id.Description = %q, want %q", idField.Description, "The unique identifier for the pet")
		}
	}
	if nameField, ok := fieldMap["name"]; ok {
		if nameField.Description != "The display name of the pet" {
			t.Errorf("Pet.name.Description = %q, want %q", nameField.Description, "The display name of the pet")
		}
	}

	// tag is type: ["string", "null"] -- should be nullable.
	if tagField, ok := fieldMap["tag"]; ok {
		if tagField.Type != "*string" {
			t.Errorf("Pet.tag: expected *string for nullable, got %s", tagField.Type)
		}
		if !tagField.IsPointer {
			t.Error("Pet.tag: expected IsPointer=true")
		}
	} else {
		t.Error("Pet: missing tag field")
	}

	// PetStatus should be an enum.
	petStatus, ok := typeMap["PetStatus"]
	if !ok {
		t.Fatal("expected PetStatus type")
	}
	if petStatus.Kind != ir.TypeKindEnum {
		t.Errorf("PetStatus: expected enum, got kind %d", petStatus.Kind)
	}
	if len(petStatus.EnumValues) != 3 {
		t.Errorf("PetStatus: expected 3 enum values, got %d", len(petStatus.EnumValues))
	}
	if petStatus.Description != "The current status of the pet in the store" {
		t.Errorf("PetStatus.Description = %q, want %q", petStatus.Description, "The current status of the pet in the store")
	}

	// PetList should be a struct with an items field that is an array.
	petList, ok := typeMap["PetList"]
	if !ok {
		t.Fatal("expected PetList type")
	}
	if petList.Kind != ir.TypeKindStruct {
		t.Errorf("PetList: expected struct, got kind %d", petList.Kind)
	}

	// Error should be a struct.
	errType, ok := typeMap["Error"]
	if !ok {
		t.Fatal("expected Error type")
	}
	if errType.Kind != ir.TypeKindStruct {
		t.Errorf("Error: expected struct, got kind %d", errType.Kind)
	}
}

func TestAnalyzeNilComponents(t *testing.T) {
	a := New(&v3high.Document{})
	pkg, err := a.Analyze("test")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(pkg.Types) != 0 {
		t.Errorf("expected no types for nil components, got %d", len(pkg.Types))
	}
}

func TestAnalyzeEmptyComponents(t *testing.T) {
	result, err := parser.Parse(filepath.Join(projectRoot(), "testdata", "petstore.yaml"), parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Temporarily nil out schemas to test the empty path.
	origSchemas := result.Model.Components.Schemas
	result.Model.Components.Schemas = nil

	a := New(result.Model)
	pkg, err := a.Analyze("test")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(pkg.Types) != 0 {
		t.Errorf("expected no types for nil schemas, got %d", len(pkg.Types))
	}

	result.Model.Components.Schemas = origSchemas
}

func TestGoTypeForPrimitive(t *testing.T) {
	tests := []struct {
		typeName string
		format   string
		want     string
	}{
		{"string", "", "string"},
		{"string", "date-time", "time.Time"},
		{"string", "byte", "[]byte"},
		{"string", "binary", "[]byte"},
		{"string", "date", "string"},
		{"integer", "", "int64"},
		{"integer", "int32", "int32"},
		{"integer", "int64", "int64"},
		{"number", "", "float64"},
		{"number", "float", "float32"},
		{"number", "double", "float64"},
		{"boolean", "", "bool"},
		{"unknown", "", "any"},
	}

	for _, tt := range tests {
		t.Run(tt.typeName+"/"+tt.format, func(t *testing.T) {
			got := goTypeForPrimitive(tt.typeName, tt.format)
			if got != tt.want {
				t.Errorf("goTypeForPrimitive(%q, %q) = %q, want %q", tt.typeName, tt.format, got, tt.want)
			}
		})
	}
}

func TestRefToSchemaName(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"#/components/schemas/Pet", "Pet"},
		{"#/components/schemas/PetStatus", "PetStatus"},
		{"#/other/path", ""},
		{"", ""},
		{"#/components/schemas/", ""},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got := refToSchemaName(tt.ref)
			if got != tt.want {
				t.Errorf("refToSchemaName(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}

func TestIsSliceType(t *testing.T) {
	if !isSliceType("[]string") {
		t.Error("expected []string to be slice type")
	}
	if !isSliceType("[]Pet") {
		t.Error("expected []Pet to be slice type")
	}
	if isSliceType("string") {
		t.Error("expected string to not be slice type")
	}
	if isSliceType("") {
		t.Error("expected empty string to not be slice type")
	}
}

func TestIsMapType(t *testing.T) {
	if !isMapType("map[string]any") {
		t.Error("expected map[string]any to be map type")
	}
	if isMapType("string") {
		t.Error("expected string to not be map type")
	}
	if isMapType("") {
		t.Error("expected empty string to not be map type")
	}
}

func TestPrimaryType(t *testing.T) {
	tests := []struct {
		name  string
		types []string
		want  string
	}{
		{"single string", []string{"string"}, "string"},
		{"nullable string", []string{"string", "null"}, "string"},
		{"null first", []string{"null", "integer"}, "integer"},
		{"only null", []string{"null"}, ""},
		{"empty", []string{}, ""},
		{"boolean", []string{"boolean"}, "boolean"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &highbase.Schema{Type: tt.types}
			got := primaryType(schema)
			if got != tt.want {
				t.Errorf("primaryType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsNullable(t *testing.T) {
	boolTrue := true
	boolFalse := false

	tests := []struct {
		name     string
		types    []string
		nullable *bool
		want     bool
	}{
		{"type array with null", []string{"string", "null"}, nil, true},
		{"type array without null", []string{"string"}, nil, false},
		{"3.0 nullable true", []string{"string"}, &boolTrue, true},
		{"3.0 nullable false", []string{"string"}, &boolFalse, false},
		{"no type no nullable", []string{}, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &highbase.Schema{
				Type:     tt.types,
				Nullable: tt.nullable,
			}
			got := isNullable(schema)
			if got != tt.want {
				t.Errorf("isNullable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzePetstore_FieldDetails(t *testing.T) {
	specPath := filepath.Join(projectRoot(), "testdata", "petstore.yaml")
	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	a := New(result.Model)
	pkg, err := a.Analyze("petstore")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	typeMap := make(map[string]*ir.TypeDef)
	for _, td := range pkg.Types {
		typeMap[td.Name] = td
	}

	// Verify CreatePetRequest fields.
	cpr := typeMap["CreatePetRequest"]
	if cpr == nil {
		t.Fatal("CreatePetRequest not found")
	}
	fieldMap := make(map[string]*ir.Field)
	for _, f := range cpr.Fields {
		fieldMap[f.JSONName] = f
	}
	if nameField, ok := fieldMap["name"]; ok {
		if nameField.Type != "string" {
			t.Errorf("CreatePetRequest.name type = %q, want string", nameField.Type)
		}
		if !nameField.Required {
			t.Error("CreatePetRequest.name should be required")
		}
	} else {
		t.Error("CreatePetRequest missing name field")
	}
	if tagField, ok := fieldMap["tag"]; ok {
		if tagField.Type != "*string" {
			t.Errorf("CreatePetRequest.tag type = %q, want *string", tagField.Type)
		}
		if tagField.Required {
			t.Error("CreatePetRequest.tag should not be required")
		}
		if !tagField.OmitEmpty {
			t.Error("CreatePetRequest.tag should have OmitEmpty")
		}
	} else {
		t.Error("CreatePetRequest missing tag field")
	}

	// Verify PetList items is []Pet array reference.
	petList := typeMap["PetList"]
	if petList == nil {
		t.Fatal("PetList not found")
	}
	plFieldMap := make(map[string]*ir.Field)
	for _, f := range petList.Fields {
		plFieldMap[f.JSONName] = f
	}
	if itemsField, ok := plFieldMap["items"]; ok {
		if itemsField.Type != "[]Pet" {
			t.Errorf("PetList.items type = %q, want []Pet", itemsField.Type)
		}
		if !itemsField.Required {
			t.Error("PetList.items should be required")
		}
	} else {
		t.Error("PetList missing items field")
	}
	if nextCursor, ok := plFieldMap["nextCursor"]; ok {
		if nextCursor.Type != "*string" {
			t.Errorf("PetList.nextCursor type = %q, want *string", nextCursor.Type)
		}
	} else {
		t.Error("PetList missing nextCursor field")
	}

	// Verify Error struct fields.
	errType := typeMap["Error"]
	if errType == nil {
		t.Fatal("Error type not found")
	}
	errFieldMap := make(map[string]*ir.Field)
	for _, f := range errType.Fields {
		errFieldMap[f.JSONName] = f
	}
	if codeField, ok := errFieldMap["code"]; ok {
		if codeField.Type != "int32" {
			t.Errorf("Error.code type = %q, want int32", codeField.Type)
		}
	} else {
		t.Error("Error missing code field")
	}
	if msgField, ok := errFieldMap["message"]; ok {
		if msgField.Type != "string" {
			t.Errorf("Error.message type = %q, want string", msgField.Type)
		}
	} else {
		t.Error("Error missing message field")
	}

	// Verify field descriptions are extracted from property schemas.
	if nameField, ok := fieldMap["name"]; ok {
		if nameField.Description != "The name of the pet to create" {
			t.Errorf("CreatePetRequest.name.Description = %q, want %q", nameField.Description, "The name of the pet to create")
		}
	}
	if tagField, ok := fieldMap["tag"]; ok {
		if tagField.Description != "An optional tag for the pet" {
			t.Errorf("CreatePetRequest.tag.Description = %q, want %q", tagField.Description, "An optional tag for the pet")
		}
	}

	// PetStatus enum values.
	petStatus := typeMap["PetStatus"]
	if petStatus == nil {
		t.Fatal("PetStatus type not found")
	}
	if petStatus.EnumGoType != "string" {
		t.Errorf("PetStatus.EnumGoType = %q, want string", petStatus.EnumGoType)
	}
	expectedEnumVals := []string{`"available"`, `"pending"`, `"sold"`}
	if len(petStatus.EnumValues) != 3 {
		t.Fatalf("PetStatus expected 3 enum values, got %d", len(petStatus.EnumValues))
	}
	for i, ev := range petStatus.EnumValues {
		if ev.Literal != expectedEnumVals[i] {
			t.Errorf("PetStatus enum literal %d = %q, want %q", i, ev.Literal, expectedEnumVals[i])
		}
	}
}

func TestAnalyzePetstore_TypeOrder(t *testing.T) {
	specPath := filepath.Join(projectRoot(), "testdata", "petstore.yaml")
	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	a := New(result.Model)
	pkg, err := a.Analyze("petstore")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// Types should be in spec-defined order.
	expectedOrder := []string{"Pet", "PetStatus", "PetList", "CreatePetRequest", "Error"}
	if len(pkg.Types) != len(expectedOrder) {
		t.Fatalf("expected %d types, got %d", len(expectedOrder), len(pkg.Types))
	}
	for i, td := range pkg.Types {
		if td.Name != expectedOrder[i] {
			t.Errorf("type[%d] = %q, want %q", i, td.Name, expectedOrder[i])
		}
	}
}

// parseComplexSchemas is a helper that parses complex-schemas.yaml and returns
// the analyzed package and a map of type name -> TypeDef.
func parseComplexSchemas(t *testing.T) (*ir.Package, map[string]*ir.TypeDef) {
	t.Helper()
	specPath := filepath.Join(projectRoot(), "testdata", "complex-schemas.yaml")
	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	a := New(result.Model)
	pkg, err := a.Analyze("complex")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	typeMap := make(map[string]*ir.TypeDef)
	for _, td := range pkg.Types {
		typeMap[td.Name] = td
	}
	return pkg, typeMap
}

func TestAllOf_DogExtendsAnimal(t *testing.T) {
	_, typeMap := parseComplexSchemas(t)

	// Animal should be a plain struct.
	animal := typeMap["Animal"]
	if animal == nil {
		t.Fatal("Animal type not found")
	}
	if animal.Kind != ir.TypeKindStruct {
		t.Errorf("Animal.Kind = %v, want struct", animal.Kind)
	}

	// Dog should be a struct with an embedded Animal + inline fields.
	dog := typeMap["Dog"]
	if dog == nil {
		t.Fatal("Dog type not found")
	}
	if dog.Kind != ir.TypeKindStruct {
		t.Errorf("Dog.Kind = %v, want struct", dog.Kind)
	}

	// Should have at least 3 fields: embedded Animal, breed, isGoodBoy.
	if len(dog.Fields) < 3 {
		t.Fatalf("Dog: expected at least 3 fields, got %d", len(dog.Fields))
	}

	// Find embedded Animal field.
	var embeddedField *ir.Field
	fieldMap := make(map[string]*ir.Field)
	for _, f := range dog.Fields {
		if f.Embedded {
			embeddedField = f
		} else {
			fieldMap[f.JSONName] = f
		}
		t.Logf("  Dog.%s: type=%s embedded=%v required=%v", f.Name, f.Type, f.Embedded, f.Required)
	}

	if embeddedField == nil {
		t.Fatal("Dog: expected an embedded field")
	}
	if embeddedField.Type != "Animal" {
		t.Errorf("Dog embedded field type = %q, want Animal", embeddedField.Type)
	}
	if embeddedField.JSONName != "" {
		t.Errorf("Dog embedded field JSONName = %q, want empty", embeddedField.JSONName)
	}

	// Check breed field (required from allOf inline required array).
	breedField, ok := fieldMap["breed"]
	if !ok {
		t.Fatal("Dog: missing breed field")
	}
	if breedField.Type != "string" {
		t.Errorf("Dog.breed type = %q, want string", breedField.Type)
	}
	if !breedField.Required {
		t.Error("Dog.breed should be required")
	}

	// Check isGoodBoy field (optional).
	isGoodBoyField, ok := fieldMap["isGoodBoy"]
	if !ok {
		t.Fatal("Dog: missing isGoodBoy field")
	}
	if isGoodBoyField.Type != "*bool" {
		t.Errorf("Dog.isGoodBoy type = %q, want *bool", isGoodBoyField.Type)
	}
}

func TestAllOf_MultipleRefs(t *testing.T) {
	_, typeMap := parseComplexSchemas(t)

	// TimestampedDog: allOf with Dog ref + Timestamped ref + inline properties.
	td := typeMap["TimestampedDog"]
	if td == nil {
		t.Fatal("TimestampedDog type not found")
	}
	if td.Kind != ir.TypeKindStruct {
		t.Errorf("TimestampedDog.Kind = %v, want struct", td.Kind)
	}

	var embeddedNames []string
	inlineFields := make(map[string]*ir.Field)
	for _, f := range td.Fields {
		if f.Embedded {
			embeddedNames = append(embeddedNames, f.Name)
		} else {
			inlineFields[f.JSONName] = f
		}
	}

	// Should have 2 embedded fields: Dog and Timestamped.
	if len(embeddedNames) != 2 {
		t.Fatalf("TimestampedDog: expected 2 embedded fields, got %d: %v", len(embeddedNames), embeddedNames)
	}

	// Should have registrationId inline field.
	if _, ok := inlineFields["registrationId"]; !ok {
		t.Error("TimestampedDog: missing registrationId field")
	}
}

func TestOneOf_ShapeWithDiscriminator(t *testing.T) {
	_, typeMap := parseComplexSchemas(t)

	shape := typeMap["Shape"]
	if shape == nil {
		t.Fatal("Shape type not found")
	}
	if shape.Kind != ir.TypeKindUnion {
		t.Errorf("Shape.Kind = %v, want union", shape.Kind)
	}
	if len(shape.UnionTypes) != 2 {
		t.Fatalf("Shape: expected 2 union variants, got %d", len(shape.UnionTypes))
	}

	// Verify variant names.
	variantNames := make(map[string]bool)
	for _, v := range shape.UnionTypes {
		variantNames[v.TypeName] = true
		t.Logf("  Shape variant: %s (disc=%q)", v.TypeName, v.DiscriminatorValue)
	}
	if !variantNames["Circle"] {
		t.Error("Shape: missing Circle variant")
	}
	if !variantNames["Rectangle"] {
		t.Error("Shape: missing Rectangle variant")
	}

	// Verify discriminator.
	if shape.Discriminator == nil {
		t.Fatal("Shape: expected discriminator")
	}
	if shape.Discriminator.PropertyName != "shapeType" {
		t.Errorf("Shape.Discriminator.PropertyName = %q, want shapeType", shape.Discriminator.PropertyName)
	}
	if len(shape.Discriminator.Mapping) != 2 {
		t.Fatalf("Shape.Discriminator.Mapping: expected 2 entries, got %d", len(shape.Discriminator.Mapping))
	}
	if shape.Discriminator.Mapping["circle"] != "Circle" {
		t.Errorf("Discriminator.Mapping[circle] = %q, want Circle", shape.Discriminator.Mapping["circle"])
	}
	if shape.Discriminator.Mapping["rectangle"] != "Rectangle" {
		t.Errorf("Discriminator.Mapping[rectangle] = %q, want Rectangle", shape.Discriminator.Mapping["rectangle"])
	}

	// Verify discriminator values on variants.
	for _, v := range shape.UnionTypes {
		if v.TypeName == "Circle" && v.DiscriminatorValue != "circle" {
			t.Errorf("Circle variant discriminator value = %q, want circle", v.DiscriminatorValue)
		}
		if v.TypeName == "Rectangle" && v.DiscriminatorValue != "rectangle" {
			t.Errorf("Rectangle variant discriminator value = %q, want rectangle", v.DiscriminatorValue)
		}
	}
}

func TestAnyOf_StringOrInt(t *testing.T) {
	_, typeMap := parseComplexSchemas(t)

	soi := typeMap["StringOrInt"]
	if soi == nil {
		t.Fatal("StringOrInt type not found")
	}
	if soi.Kind != ir.TypeKindUnion {
		t.Errorf("StringOrInt.Kind = %v, want union", soi.Kind)
	}
	if len(soi.UnionTypes) != 2 {
		t.Fatalf("StringOrInt: expected 2 union variants, got %d", len(soi.UnionTypes))
	}
	// No discriminator for anyOf without one.
	if soi.Discriminator != nil {
		t.Error("StringOrInt: expected no discriminator for anyOf")
	}
}

func TestAdditionalProperties_MapBoolTrue(t *testing.T) {
	_, typeMap := parseComplexSchemas(t)

	// Metadata: additionalProperties: true, no properties -> map[string]any alias.
	meta := typeMap["Metadata"]
	if meta == nil {
		t.Fatal("Metadata type not found")
	}
	if meta.Kind != ir.TypeKindAlias {
		t.Errorf("Metadata.Kind = %v, want alias", meta.Kind)
	}
	if meta.GoType != "map[string]any" {
		t.Errorf("Metadata.GoType = %q, want map[string]any", meta.GoType)
	}
}

func TestAdditionalProperties_TypedSchema(t *testing.T) {
	_, typeMap := parseComplexSchemas(t)

	// Labels: additionalProperties with type: string -> map[string]string alias.
	labels := typeMap["Labels"]
	if labels == nil {
		t.Fatal("Labels type not found")
	}
	if labels.Kind != ir.TypeKindAlias {
		t.Errorf("Labels.Kind = %v, want alias", labels.Kind)
	}
	if labels.GoType != "map[string]string" {
		t.Errorf("Labels.GoType = %q, want map[string]string", labels.GoType)
	}
}

func TestAdditionalProperties_WithProperties(t *testing.T) {
	_, typeMap := parseComplexSchemas(t)

	// Config: has both properties AND additionalProperties.
	config := typeMap["Config"]
	if config == nil {
		t.Fatal("Config type not found")
	}
	if config.Kind != ir.TypeKindStruct {
		t.Errorf("Config.Kind = %v, want struct", config.Kind)
	}

	fieldMap := make(map[string]*ir.Field)
	for _, f := range config.Fields {
		fieldMap[f.Name] = f
		t.Logf("  Config.%s: type=%s json=%q", f.Name, f.Type, f.JSONName)
	}

	// Should have name, version, and AdditionalProperties fields.
	if nameField, ok := fieldMap["Name"]; !ok {
		t.Error("Config: missing Name field")
	} else if nameField.Type != "string" {
		t.Errorf("Config.Name type = %q, want string", nameField.Type)
	}

	if versionField, ok := fieldMap["Version"]; !ok {
		t.Error("Config: missing Version field")
	} else if versionField.Type != "*string" {
		t.Errorf("Config.Version type = %q, want *string", versionField.Type)
	}

	apField, ok := fieldMap["AdditionalProperties"]
	if !ok {
		t.Fatal("Config: missing AdditionalProperties field")
	}
	if apField.Type != "map[string]string" {
		t.Errorf("Config.AdditionalProperties type = %q, want map[string]string", apField.Type)
	}
	if !apField.CatchAll {
		t.Error("Config.AdditionalProperties should be marked CatchAll")
	}
	if apField.OmitEmpty {
		t.Error("Config.AdditionalProperties must not set OmitEmpty; the tag has to stay exactly \"-\"")
	}
}

func TestOneOf_DogOrCircle(t *testing.T) {
	_, typeMap := parseComplexSchemas(t)

	// DogOrCircle is a oneOf with refs to Dog (which is itself allOf) and Circle.
	doc := typeMap["DogOrCircle"]
	if doc == nil {
		t.Fatal("DogOrCircle type not found")
	}
	if doc.Kind != ir.TypeKindUnion {
		t.Errorf("DogOrCircle.Kind = %v, want union", doc.Kind)
	}
	if len(doc.UnionTypes) != 2 {
		t.Fatalf("DogOrCircle: expected 2 union variants, got %d", len(doc.UnionTypes))
	}
	variantNames := make(map[string]bool)
	for _, v := range doc.UnionTypes {
		variantNames[v.TypeName] = true
	}
	if !variantNames["Dog"] {
		t.Error("DogOrCircle: missing Dog variant")
	}
	if !variantNames["Circle"] {
		t.Error("DogOrCircle: missing Circle variant")
	}
}

func TestComplexSchemas_AllTypesPresent(t *testing.T) {
	pkg, _ := parseComplexSchemas(t)

	expectedTypes := []string{
		"Animal", "Dog", "Circle", "Rectangle", "Shape",
		"StringOrInt", "Metadata", "Labels", "Config",
		"DogOrCircle", "Timestamped", "TimestampedDog",
		"ShapeCollection", "ShapeCollectionShapesValue",
		// Unions used directly as a request or response body.
		"CreateShapeBody", "CreateShapeResponse", "NamedShape",
		// Circle and Rectangle both declare shapeType, and the body union
		// dispatches on kind, so the shared property becomes a base.
		"CreateShapeBodyBase",
		// StringOrInt's two members are objects written inline, each of which
		// declares a property and so names a struct.
		"StringOrIntVariant", "StringOrIntVariant2",
	}

	if len(pkg.Types) != len(expectedTypes) {
		names := make([]string, len(pkg.Types))
		for i, td := range pkg.Types {
			names[i] = td.Name
		}
		t.Fatalf("expected %d types, got %d: %v", len(expectedTypes), len(pkg.Types), names)
	}

	for _, name := range expectedTypes {
		found := false
		for _, td := range pkg.Types {
			if td.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected type %q not found", name)
		}
	}
}
