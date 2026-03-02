package generator

import (
	"strings"
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
	"golang.org/x/tools/imports"
)

func TestGenerate_StructAndEnum(t *testing.T) {
	pkg := &ir.Package{
		Name: "petstore",
		Types: []*ir.TypeDef{
			{
				Name:        "Pet",
				Description: "A pet in the store",
				Kind:        ir.TypeKindStruct,
				Fields: []*ir.Field{
					{
						Name:     "ID",
						JSONName: "id",
						Type:     "int64",
						Required: true,
					},
					{
						Name:        "Name",
						JSONName:    "name",
						Type:        "string",
						Description: "The name of the pet",
						Required:    true,
					},
					{
						Name:      "Tag",
						JSONName:  "tag",
						Type:      "*string",
						OmitEmpty: true,
					},
				},
			},
			{
				Name:       "PetStatus",
				Kind:       ir.TypeKindEnum,
				EnumGoType: "string",
				EnumValues: []*ir.EnumVal{
					{Name: "PetStatusAvailable", Value: "available"},
					{Name: "PetStatusPending", Value: "pending"},
					{Name: "PetStatusSold", Value: "sold"},
				},
			},
		},
	}

	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if len(files) < 1 {
		t.Fatalf("expected at least 1 generated file, got %d", len(files))
	}

	// Find the types.go file.
	var content string
	for _, f := range files {
		if f.Name == "types.go" {
			content = string(f.Content)
			break
		}
	}
	if content == "" {
		t.Fatal("expected types.go in generated files")
	}

	// Verify package declaration
	if !strings.Contains(content, "package petstore") {
		t.Error("output missing 'package petstore'")
	}

	// Verify struct definition
	if !strings.Contains(content, "type Pet struct") {
		t.Error("output missing 'type Pet struct'")
	}

	// Verify struct fields with JSON tags
	if !strings.Contains(content, `json:"id"`) {
		t.Error("output missing json tag for id")
	}
	if !strings.Contains(content, `json:"name"`) {
		t.Error("output missing json tag for name")
	}
	if !strings.Contains(content, `json:"tag,omitempty"`) {
		t.Error("output missing json tag with omitempty for tag")
	}

	// Verify description comment
	if !strings.Contains(content, "A pet in the store") {
		t.Error("output missing Pet description comment")
	}
	if !strings.Contains(content, "The name of the pet") {
		t.Error("output missing Name field description comment")
	}

	// Verify enum type and consts
	if !strings.Contains(content, "type PetStatus string") {
		t.Error("output missing 'type PetStatus string'")
	}
	if !strings.Contains(content, `PetStatusAvailable PetStatus = "available"`) {
		t.Error("output missing PetStatusAvailable const")
	}
	if !strings.Contains(content, `PetStatusPending PetStatus = "pending"`) {
		t.Error("output missing PetStatusPending const")
	}
	if !strings.Contains(content, `PetStatusSold PetStatus = "sold"`) {
		t.Error("output missing PetStatusSold const")
	}
}

func TestGenerate_GoimportsSucceeds(t *testing.T) {
	pkg := &ir.Package{
		Name: "testpkg",
		Types: []*ir.TypeDef{
			{
				Name: "Simple",
				Kind: ir.TypeKindStruct,
				Fields: []*ir.Field{
					{
						Name:     "Value",
						JSONName: "value",
						Type:     "string",
						Required: true,
					},
				},
			},
		},
	}

	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("expected at least 1 generated file")
	}

	// Verify goimports can process the output without error
	_, err = imports.Process(files[0].Name, files[0].Content, &imports.Options{
		Comments:  true,
		TabIndent: true,
		TabWidth:  8,
	})
	if err != nil {
		t.Errorf("goimports failed on generated output: %v\n\nRaw output:\n%s", err, string(files[0].Content))
	}
}

func TestGenerate_AliasType(t *testing.T) {
	pkg := &ir.Package{
		Name: "testpkg",
		Types: []*ir.TypeDef{
			{
				Name:        "UserID",
				Description: "A unique user identifier",
				Kind:        ir.TypeKindAlias,
				GoType:      "string",
			},
		},
	}

	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	content := string(files[0].Content)

	if !strings.Contains(content, "type UserID = string") {
		t.Errorf("output missing alias declaration, got:\n%s", content)
	}
	if !strings.Contains(content, "A unique user identifier") {
		t.Error("output missing alias description comment")
	}
}

func TestGenerate_UnionType(t *testing.T) {
	pkg := &ir.Package{
		Name: "testpkg",
		Types: []*ir.TypeDef{
			{
				Name:        "PetOrError",
				Description: "Either a Pet or an Error",
				Kind:        ir.TypeKindUnion,
				UnionTypes: []*ir.UnionVariant{
					{TypeName: "Pet"},
					{TypeName: "Error"},
				},
			},
		},
	}

	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// Find types.go content.
	var content string
	for _, f := range files {
		if f.Name == "types.go" {
			content = string(f.Content)
			break
		}
	}
	if content == "" {
		t.Fatal("expected types.go in generated files")
	}

	if !strings.Contains(content, "type PetOrError struct") {
		t.Errorf("output missing union struct declaration, got:\n%s", content)
	}
	if !strings.Contains(content, "Value any") {
		t.Errorf("output missing Value any field, got:\n%s", content)
	}
	if !strings.Contains(content, "Pet") && !strings.Contains(content, "Error") {
		t.Error("output missing variant names in comment")
	}
	if !strings.Contains(content, "Either a Pet or an Error") {
		t.Error("output missing union description comment")
	}

	// Verify MarshalJSON and UnmarshalJSON are generated.
	if !strings.Contains(content, "func (u PetOrError) MarshalJSON()") {
		t.Error("output missing MarshalJSON method")
	}
	if !strings.Contains(content, "func (u *PetOrError) UnmarshalJSON(data []byte)") {
		t.Error("output missing UnmarshalJSON method")
	}
	// Without discriminator, should try each variant.
	if !strings.Contains(content, "valPet") {
		t.Error("output missing try-each-variant logic for Pet")
	}
	if !strings.Contains(content, "valError") {
		t.Error("output missing try-each-variant logic for Error")
	}

	// Verify encoding/json and fmt imports.
	if !strings.Contains(content, `"encoding/json"`) {
		t.Error("output missing encoding/json import")
	}
	if !strings.Contains(content, `"fmt"`) {
		t.Error("output missing fmt import")
	}
}

func TestGenerate_UnionTypeWithDiscriminator(t *testing.T) {
	pkg := &ir.Package{
		Name: "testpkg",
		Types: []*ir.TypeDef{
			{
				Name:        "Shape",
				Description: "A geometric shape",
				Kind:        ir.TypeKindUnion,
				UnionTypes: []*ir.UnionVariant{
					{TypeName: "Circle", DiscriminatorValue: "circle"},
					{TypeName: "Rectangle", DiscriminatorValue: "rectangle"},
				},
				Discriminator: &ir.DiscriminatorDef{
					PropertyName: "shapeType",
					Mapping: map[string]string{
						"circle":    "Circle",
						"rectangle": "Rectangle",
					},
				},
			},
		},
	}

	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	var content string
	for _, f := range files {
		if f.Name == "types.go" {
			content = string(f.Content)
			break
		}
	}
	if content == "" {
		t.Fatal("expected types.go in generated files")
	}

	// Verify discriminator-based UnmarshalJSON.
	if !strings.Contains(content, "func (u *Shape) UnmarshalJSON(data []byte)") {
		t.Errorf("output missing UnmarshalJSON method, got:\n%s", content)
	}
	if !strings.Contains(content, `ShapeType string`) {
		t.Errorf("output missing discriminator struct field, got:\n%s", content)
	}
	if !strings.Contains(content, `case "circle"`) {
		t.Errorf("output missing discriminator case for circle, got:\n%s", content)
	}
	if !strings.Contains(content, `case "rectangle"`) {
		t.Errorf("output missing discriminator case for rectangle, got:\n%s", content)
	}
}

func TestGenerate_EmbeddedFields(t *testing.T) {
	pkg := &ir.Package{
		Name: "testpkg",
		Types: []*ir.TypeDef{
			{
				Name: "Dog",
				Kind: ir.TypeKindStruct,
				Fields: []*ir.Field{
					{
						Name:     "Animal",
						Type:     "Animal",
						Embedded: true,
					},
					{
						Name:     "Breed",
						JSONName: "breed",
						Type:     "string",
						Required: true,
					},
				},
			},
		},
	}

	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	var content string
	for _, f := range files {
		if f.Name == "types.go" {
			content = string(f.Content)
			break
		}
	}
	if content == "" {
		t.Fatal("expected types.go in generated files")
	}

	// Embedded field should appear without json tag.
	if !strings.Contains(content, "\tAnimal\n") {
		t.Errorf("output missing embedded Animal field (expected no json tag), got:\n%s", content)
	}
	// Regular field should still have json tag.
	if !strings.Contains(content, `Breed string`) {
		t.Errorf("output missing Breed field, got:\n%s", content)
	}
	if !strings.Contains(content, `json:"breed"`) {
		t.Errorf("output missing json tag for breed, got:\n%s", content)
	}
}

func TestGenerate_NoUnions_NoJsonImport(t *testing.T) {
	pkg := &ir.Package{
		Name: "testpkg",
		Types: []*ir.TypeDef{
			{
				Name: "Simple",
				Kind: ir.TypeKindStruct,
				Fields: []*ir.Field{
					{
						Name:     "Value",
						JSONName: "value",
						Type:     "string",
						Required: true,
					},
				},
			},
		},
	}

	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	var content string
	for _, f := range files {
		if f.Name == "types.go" {
			content = string(f.Content)
			break
		}
	}
	if content == "" {
		t.Fatal("expected types.go in generated files")
	}

	// Without unions, encoding/json should NOT be imported.
	if strings.Contains(content, `"encoding/json"`) {
		t.Error("output should not contain encoding/json import when no unions exist")
	}
}

func TestCleanDoc(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple description", "simple description"},
		{"line one\nline two", "line one line two"},
		{"line one\r\nline two", "line one line two"},
		{"  padded  ", "padded"},
		{"multi\n\nblank\nlines", "multi  blank lines"},
	}
	for _, tt := range tests {
		got := cleanDoc(tt.input)
		if got != tt.want {
			t.Errorf("cleanDoc(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestJsonTag(t *testing.T) {
	tests := []struct {
		field *ir.Field
		want  string
	}{
		{&ir.Field{JSONName: "name"}, "name"},
		{&ir.Field{JSONName: "tag", OmitEmpty: true}, "tag,omitempty"},
		{&ir.Field{JSONName: "id", OmitEmpty: false}, "id"},
	}
	for _, tt := range tests {
		got := jsonTag(tt.field)
		if got != tt.want {
			t.Errorf("jsonTag(%+v) = %q, want %q", tt.field, got, tt.want)
		}
	}
}

func TestHasOptionalQueryParams(t *testing.T) {
	tests := []struct {
		name string
		op   *ir.OperationDef
		want bool
	}{
		{
			name: "no params",
			op:   &ir.OperationDef{},
			want: false,
		},
		{
			name: "only required query params",
			op: &ir.OperationDef{
				QueryParams: []*ir.ParamDef{{Name: "id", Required: true}},
			},
			want: false,
		},
		{
			name: "optional query param",
			op: &ir.OperationDef{
				QueryParams: []*ir.ParamDef{{Name: "limit", Required: false}},
			},
			want: true,
		},
		{
			name: "only header params",
			op: &ir.OperationDef{
				HeaderParams: []*ir.ParamDef{{Name: "X-Request-Id", Required: false}},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasOptionalQueryParams(tt.op); got != tt.want {
				t.Errorf("hasOptionalQueryParams() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasOptionalHeaderParams(t *testing.T) {
	tests := []struct {
		name string
		op   *ir.OperationDef
		want bool
	}{
		{
			name: "no params",
			op:   &ir.OperationDef{},
			want: false,
		},
		{
			name: "only query params",
			op: &ir.OperationDef{
				QueryParams: []*ir.ParamDef{{Name: "limit", Required: false}},
			},
			want: false,
		},
		{
			name: "optional header param",
			op: &ir.OperationDef{
				HeaderParams: []*ir.ParamDef{{Name: "X-Request-Id", Required: false}},
			},
			want: true,
		},
		{
			name: "only required header params",
			op: &ir.OperationDef{
				HeaderParams: []*ir.ParamDef{{Name: "X-Request-Id", Required: true}},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasOptionalHeaderParams(tt.op); got != tt.want {
				t.Errorf("hasOptionalHeaderParams() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnumLiteral(t *testing.T) {
	tests := []struct {
		val  *ir.EnumVal
		want string
	}{
		{&ir.EnumVal{Name: "X", Value: "hello"}, `"hello"`},
		{&ir.EnumVal{Name: "X", Value: float64(42)}, "42"},
		{&ir.EnumVal{Name: "X", Value: float64(3.14)}, "3.14"},
		{&ir.EnumVal{Name: "X", Value: true}, "true"},
		{&ir.EnumVal{Name: "X", Value: int64(99)}, "99"},
	}
	for _, tt := range tests {
		got := enumLiteral(tt.val)
		if got != tt.want {
			t.Errorf("enumLiteral(%v) = %q, want %q", tt.val.Value, got, tt.want)
		}
	}
}
