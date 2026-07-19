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
				Name:        "PetStatus",
				Description: "The status of a pet",
				Kind:        ir.TypeKindEnum,
				EnumGoType:  "string",
				EnumValues: []*ir.EnumVal{
					{Name: "PetStatusAvailable", Literal: `"available"`},
					{Name: "PetStatusPending", Literal: `"pending"`},
					{Name: "PetStatusSold", Literal: `"sold"`},
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

	// Verify type-level doc comment (new format: "// Name - description")
	if !strings.Contains(content, "// Pet - A pet in the store") {
		t.Error("output missing Pet type doc comment")
	}
	if !strings.Contains(content, "The name of the pet") {
		t.Error("output missing Name field description comment")
	}

	// Verify enum type doc comment
	if !strings.Contains(content, "// PetStatus - The status of a pet") {
		t.Error("output missing PetStatus type doc comment")
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

func TestGenerate_UserAgent(t *testing.T) {
	clientContent := func(pkg *ir.Package) string {
		gen, err := New(pkg)
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}
		files, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		for _, f := range files {
			if f.Name == "client.go" {
				return string(f.Content)
			}
		}
		t.Fatal("expected client.go in generated files")
		return ""
	}

	t.Run("custom user agent", func(t *testing.T) {
		content := clientContent(&ir.Package{Name: "testpkg", UserAgent: "myproduct-sdk/2"})
		if !strings.Contains(content, `userAgent:  "myproduct-sdk/2",`) {
			t.Error("output missing custom userAgent default")
		}
	})

	t.Run("default user agent", func(t *testing.T) {
		content := clientContent(&ir.Package{Name: "testpkg"})
		if !strings.Contains(content, `userAgent:  "openapi-client-generator/1.0",`) {
			t.Error("output missing default userAgent")
		}
	})
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

func TestSuccessContentType(t *testing.T) {
	tests := []struct {
		name string
		op   *ir.OperationDef
		want string
	}{
		{
			name: "nil success response",
			op:   &ir.OperationDef{},
			want: "application/json",
		},
		{
			name: "empty content type",
			op: &ir.OperationDef{
				SuccessResponse: &ir.ResponseDef{TypeName: "string"},
			},
			want: "application/json",
		},
		{
			name: "application/json",
			op: &ir.OperationDef{
				SuccessResponse: &ir.ResponseDef{ContentType: "application/json", TypeName: "Pet"},
			},
			want: "application/json",
		},
		{
			name: "text/plain",
			op: &ir.OperationDef{
				SuccessResponse: &ir.ResponseDef{ContentType: "text/plain", TypeName: "string"},
			},
			want: "text/plain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := successContentType(tt.op); got != tt.want {
				t.Errorf("successContentType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTypeDocComment(t *testing.T) {
	tests := []struct {
		name string
		td   *ir.TypeDef
		want string
	}{
		{
			name: "empty description",
			td:   &ir.TypeDef{Name: "Pet", Description: ""},
			want: "",
		},
		{
			name: "single line",
			td:   &ir.TypeDef{Name: "Pet", Description: "A pet in the store"},
			want: "// Pet - A pet in the store",
		},
		{
			name: "multi line",
			td:   &ir.TypeDef{Name: "Pet", Description: "A pet in the store\nWith extra info"},
			want: "// Pet - A pet in the store\n// With extra info",
		},
		{
			name: "whitespace only",
			td:   &ir.TypeDef{Name: "Pet", Description: "  "},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := typeDocComment(tt.td)
			if got != tt.want {
				t.Errorf("typeDocComment() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOpDocComment(t *testing.T) {
	tests := []struct {
		name string
		op   *ir.OperationDef
		want string
	}{
		{
			name: "empty",
			op:   &ir.OperationDef{Name: "ListPets"},
			want: "",
		},
		{
			name: "summary only",
			op:   &ir.OperationDef{Name: "ListPets", Summary: "List all pets"},
			want: "// ListPets - List all pets",
		},
		{
			name: "description only",
			op:   &ir.OperationDef{Name: "ListPets", Description: "Returns all pets"},
			want: "// ListPets - Returns all pets",
		},
		{
			name: "summary and description",
			op:   &ir.OperationDef{Name: "ListPets", Summary: "List all pets", Description: "Returns a paginated list of all pets."},
			want: "// ListPets - List all pets\n//\n// Returns a paginated list of all pets.",
		},
		{
			name: "deprecated only",
			op:   &ir.OperationDef{Name: "ListPets", Deprecated: true},
			want: "// Deprecated: this operation is deprecated.",
		},
		{
			name: "summary and deprecated",
			op:   &ir.OperationDef{Name: "ListPets", Summary: "List all pets", Deprecated: true},
			want: "// ListPets - List all pets\n//\n// Deprecated: this operation is deprecated.",
		},
		{
			name: "multi-line description",
			op:   &ir.OperationDef{Name: "ListPets", Description: "Returns pets.\nSupports pagination."},
			want: "// ListPets - Returns pets.\n// Supports pagination.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := opDocComment(tt.op)
			if got != tt.want {
				t.Errorf("opDocComment() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFieldDocComment(t *testing.T) {
	tests := []struct {
		name  string
		field *ir.Field
		want  string
	}{
		{
			name:  "empty",
			field: &ir.Field{Name: "ID"},
			want:  "",
		},
		{
			name:  "description only",
			field: &ir.Field{Name: "ID", Description: "The unique ID"},
			want:  "// The unique ID",
		},
		{
			name:  "deprecated only",
			field: &ir.Field{Name: "ID", Deprecated: true},
			want:  "// Deprecated: this field is deprecated.",
		},
		{
			name:  "description and deprecated",
			field: &ir.Field{Name: "ID", Description: "The unique ID", Deprecated: true},
			want:  "// The unique ID\n//\n// Deprecated: this field is deprecated.",
		},
		{
			name:  "multi-line description",
			field: &ir.Field{Name: "ID", Description: "The unique ID.\nMust be positive."},
			want:  "// The unique ID.\n// Must be positive.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fieldDocComment(tt.field)
			if got != tt.want {
				t.Errorf("fieldDocComment() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParamDocComment(t *testing.T) {
	tests := []struct {
		name  string
		param *ir.ParamDef
		want  string
	}{
		{
			name:  "empty",
			param: &ir.ParamDef{Name: "limit"},
			want:  "",
		},
		{
			name:  "description only",
			param: &ir.ParamDef{Name: "limit", Description: "Max items to return"},
			want:  "// Max items to return",
		},
		{
			name:  "deprecated only",
			param: &ir.ParamDef{Name: "limit", Deprecated: true},
			want:  "// Deprecated: this parameter is deprecated.",
		},
		{
			name:  "description and deprecated",
			param: &ir.ParamDef{Name: "limit", Description: "Max items to return", Deprecated: true},
			want:  "// Max items to return\n//\n// Deprecated: this parameter is deprecated.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := paramDocComment(tt.param)
			if got != tt.want {
				t.Errorf("paramDocComment() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIndent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"single line", "// hello", "\t// hello"},
		{"multi line", "// line1\n// line2", "\t// line1\n\t// line2"},
		{"with blank line", "// line1\n//\n// line2", "\t// line1\n\t//\n\t// line2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := indent(tt.input)
			if got != tt.want {
				t.Errorf("indent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerate_DeprecatedOperation(t *testing.T) {
	pkg := &ir.Package{
		Name: "testpkg",
		Types: []*ir.TypeDef{
			{Name: "Simple", Kind: ir.TypeKindStruct, Fields: []*ir.Field{
				{Name: "Value", JSONName: "value", Type: "string", Required: true},
			}},
		},
		Operations: []*ir.OperationDef{
			{
				Name:       "OldMethod",
				Summary:    "An old method",
				HTTPMethod: "GET",
				Path:       "/old",
				Deprecated: true,
				SuccessResponse: &ir.ResponseDef{
					StatusCode:  "200",
					ContentType: "application/json",
					TypeName:    "Simple",
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

	var opsContent string
	for _, f := range files {
		if f.Name == "operations.go" {
			opsContent = string(f.Content)
			break
		}
	}
	if opsContent == "" {
		t.Fatal("expected operations.go in generated files")
	}

	if !strings.Contains(opsContent, "// Deprecated: this operation is deprecated.") {
		t.Error("output missing Deprecated marker in operation doc comment")
	}
	if !strings.Contains(opsContent, "// OldMethod - An old method") {
		t.Error("output missing operation summary in doc comment")
	}
}

func TestGenerate_FieldDeprecated(t *testing.T) {
	pkg := &ir.Package{
		Name: "testpkg",
		Types: []*ir.TypeDef{
			{
				Name: "Item",
				Kind: ir.TypeKindStruct,
				Fields: []*ir.Field{
					{
						Name:        "OldField",
						JSONName:    "oldField",
						Type:        "string",
						Description: "This field is old",
						Deprecated:  true,
						Required:    true,
					},
					{
						Name:     "NewField",
						JSONName: "newField",
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

	if !strings.Contains(content, "// Deprecated: this field is deprecated.") {
		t.Error("output missing Deprecated marker for OldField")
	}
	if !strings.Contains(content, "// This field is old") {
		t.Error("output missing description for deprecated field")
	}
}

func TestGenerate_ParamDescriptions(t *testing.T) {
	pkg := &ir.Package{
		Name: "testpkg",
		Types: []*ir.TypeDef{
			{Name: "Result", Kind: ir.TypeKindStruct, Fields: []*ir.Field{
				{Name: "Data", JSONName: "data", Type: "string", Required: true},
			}},
		},
		Operations: []*ir.OperationDef{
			{
				Name:       "Search",
				Summary:    "Search items",
				HTTPMethod: "GET",
				Path:       "/search",
				QueryParams: []*ir.ParamDef{
					{
						Name:        "query",
						FieldName:   "Query",
						OrigName:    "query",
						Location:    "query",
						Type:        "string",
						Required:    false,
						Description: "The search query string",
					},
					{
						Name:        "limit",
						FieldName:   "Limit",
						OrigName:    "limit",
						Location:    "query",
						Type:        "int32",
						Required:    false,
						Description: "Maximum number of results",
					},
				},
				SuccessResponse: &ir.ResponseDef{
					StatusCode:  "200",
					ContentType: "application/json",
					TypeName:    "Result",
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

	var opsContent string
	for _, f := range files {
		if f.Name == "operations.go" {
			opsContent = string(f.Content)
			break
		}
	}
	if opsContent == "" {
		t.Fatal("expected operations.go in generated files")
	}

	if !strings.Contains(opsContent, "// The search query string") {
		t.Error("output missing query param description")
	}
	if !strings.Contains(opsContent, "// Maximum number of results") {
		t.Error("output missing limit param description")
	}
}

func TestGenerate_RetriesTransientNetworkErrors(t *testing.T) {
	pkg := &ir.Package{
		Name: "testpkg",
		Types: []*ir.TypeDef{
			{
				Name: "Simple",
				Kind: ir.TypeKindStruct,
				Fields: []*ir.Field{
					{Name: "Value", JSONName: "value", Type: "string", Required: true},
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

	var clientContent, retryContent string
	for _, f := range files {
		switch f.Name {
		case "client.go":
			clientContent = string(f.Content)
		case "retry.go":
			retryContent = string(f.Content)
		}
	}
	if clientContent == "" {
		t.Fatal("expected client.go in generated files")
	}
	if retryContent == "" {
		t.Fatal("expected retry.go in generated files")
	}

	// retry.go must define the transient-network-error helpers.
	if !strings.Contains(retryContent, "func isIdempotentMethod(method string) bool") {
		t.Error("retry.go missing isIdempotentMethod helper")
	}
	if !strings.Contains(retryContent, "func isRetryableNetworkError(err error) bool") {
		t.Error("retry.go missing isRetryableNetworkError helper")
	}

	// client.go must retry idempotent requests on transient network errors
	// rather than returning immediately.
	if !strings.Contains(clientContent, "isIdempotentMethod(method) && isRetryableNetworkError(err)") {
		t.Error("client.go missing network-error retry path")
	}
	if strings.Contains(clientContent, "// Network errors are not retryable.") {
		t.Error("client.go still treats all network errors as non-retryable")
	}

	// Status-code retries must be gated by method so non-idempotent requests
	// (POST/PATCH) aren't replayed on 5xx.
	if !strings.Contains(retryContent, "func shouldRetryStatus(method string, statusCode int, cfg RetryConfig) bool") {
		t.Error("retry.go missing shouldRetryStatus helper")
	}
	if !strings.Contains(clientContent, "shouldRetryStatus(method, resp.StatusCode, *c.retryConfig)") {
		t.Error("client.go status-retry not gated by method")
	}
	if strings.Contains(clientContent, "shouldRetry(resp.StatusCode, *c.retryConfig)") {
		t.Error("client.go still retries all methods on retryable status codes")
	}
}
