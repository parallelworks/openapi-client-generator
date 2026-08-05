package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
	"github.com/parallelworks/openapi-client-generator/internal/parser"
)

// analyzeSpec analyzes an inline spec and returns the package alongside its
// types keyed by Go name.
func analyzeSpec(t *testing.T, spec string) (*ir.Package, map[string]*ir.TypeDef) {
	t.Helper()
	specPath := filepath.Join(t.TempDir(), "spec.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("writing spec: %v", err)
	}
	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	pkg, err := New(result.Model).Analyze("test")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	typeMap := make(map[string]*ir.TypeDef, len(pkg.Types))
	for _, td := range pkg.Types {
		typeMap[td.Name] = td
	}
	return pkg, typeMap
}

const nullableUnionSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /recipes:
    get:
      operationId: listRecipes
      parameters:
        - name: search
          in: query
          required: false
          schema:
            anyOf: [{ type: string }, { type: "null" }]
            title: Search
        - name: limit
          in: query
          required: true
          schema:
            anyOf: [{ type: integer, format: int32 }, { type: "null" }]
        - name: owner
          in: query
          required: false
          schema:
            anyOf: [{ $ref: "#/components/schemas/Person" }, { type: "null" }]
        - name: either
          in: query
          required: false
          schema:
            anyOf: [{ type: string }, { type: integer }, { type: "null" }]
        - name: cookbook
          in: query
          required: false
          schema:
            anyOf:
              - { type: string, format: uuid4 }
              - { type: string }
              - { type: "null" }
        - name: categories
          in: query
          required: false
          schema:
            anyOf:
              - type: array
                items:
                  anyOf: [{ type: string, format: uuid4 }, { type: string }]
              - { type: "null" }
      responses:
        "200": { description: ok }
components:
  schemas:
    Person:
      type: object
      properties:
        name: { type: string }
    MaybeName:
      anyOf: [{ type: string }, { type: "null" }]
    MaybePerson:
      oneOf: [{ $ref: "#/components/schemas/Person" }, { type: "null" }]
    Recipe:
      type: object
      required: [name]
      properties:
        name:
          anyOf: [{ type: string }, { type: "null" }]
        tags:
          anyOf: [{ type: array, items: { type: string } }, { type: "null" }]
        cook:
          anyOf: [{ $ref: "#/components/schemas/Person" }, { type: "null" }]
`

// TestRequestBodyContentType checks which media type an operation sends its body
// as, now that the choice decides the encoding rather than just a header.
func TestRequestBodyContentType(t *testing.T) {
	pkg, _ := analyzeSpec(t, `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /upload:
    post:
      operationId: upload
      requestBody:
        content:
          multipart/form-data:
            schema: { type: object }
      responses: { "204": { description: ok } }
  /both:
    post:
      operationId: both
      requestBody:
        content:
          multipart/form-data:
            schema: { type: object }
          application/json:
            schema: { type: object }
      responses: { "204": { description: ok } }
  /raw:
    post:
      operationId: raw
      requestBody:
        content:
          application/octet-stream:
            schema: { type: string, format: binary }
      responses: { "204": { description: ok } }
`)

	byName := make(map[string]*ir.OperationDef, len(pkg.Operations))
	for _, op := range pkg.Operations {
		byName[op.Name] = op
	}

	tests := []struct {
		op   string
		want string
	}{
		{"Upload", "multipart/form-data"},
		// JSON wins when the spec offers a choice, whatever order it lists them in.
		{"Both", "application/json"},
		{"Raw", "application/octet-stream"},
	}
	for _, tt := range tests {
		op := byName[tt.op]
		if op == nil {
			t.Errorf("operation %q not found", tt.op)
			continue
		}
		if op.RequestBody == nil {
			t.Errorf("%s has no request body", tt.op)
			continue
		}
		if op.RequestBody.ContentType != tt.want {
			t.Errorf("%s content type = %q, want %q", tt.op, op.RequestBody.ContentType, tt.want)
		}
	}
}

// TestNullableUnion_QueryParamsResolveToTheVariantType covers issue #16: a query
// parameter typed `anyOf: [{type: string}, {type: "null"}]` used to land in the
// params struct as *any, which callers had no way to construct a value for.
func TestNullableUnion_QueryParamsResolveToTheVariantType(t *testing.T) {
	pkg, _ := analyzeSpec(t, nullableUnionSpec)

	if len(pkg.Operations) != 1 {
		t.Fatalf("operations = %d, want 1", len(pkg.Operations))
	}
	params := make(map[string]*ir.ParamDef)
	for _, p := range pkg.Operations[0].QueryParams {
		params[p.OrigName] = p
	}

	tests := []struct {
		param string
		want  string
	}{
		{"search", "string"},
		{"limit", "int32"},
		{"owner", "Person"},
		// Variants that are refinements of one Go type collapse to that type.
		{"cookbook", "string"},
		{"categories", "[]string"},
		// A union with a real choice keeps its union handling.
		{"either", "any"},
	}
	for _, tt := range tests {
		p := params[tt.param]
		if p == nil {
			t.Errorf("query param %q not found", tt.param)
			continue
		}
		if p.Type != tt.want {
			t.Errorf("param %q type = %q, want %q", tt.param, p.Type, tt.want)
		}
	}
}

// TestNullableUnion_ComponentSchemasCollapse checks the same collapse when the
// union is a named component schema rather than an inline parameter schema.
func TestNullableUnion_ComponentSchemasCollapse(t *testing.T) {
	_, typeMap := analyzeSpec(t, nullableUnionSpec)

	name := typeMap["MaybeName"]
	if name == nil {
		t.Fatal("MaybeName type not found")
	}
	if name.Kind != ir.TypeKindAlias || name.GoType != "string" {
		t.Errorf("MaybeName = %v %q, want alias string", name.Kind, name.GoType)
	}
	if !name.IsNullable {
		t.Error("MaybeName.IsNullable = false, want true")
	}

	// A $ref variant aliases the type it points at instead of restating it.
	person := typeMap["MaybePerson"]
	if person == nil {
		t.Fatal("MaybePerson type not found")
	}
	if person.Kind != ir.TypeKindAlias || person.GoType != "Person" {
		t.Errorf("MaybePerson = %v %q, want alias Person", person.Kind, person.GoType)
	}
}

// TestNullableUnion_StructFieldsArePointersToTheVariantType checks that a
// collapsed property still carries the null through a pointer, including when
// the property is required.
func TestNullableUnion_StructFieldsArePointersToTheVariantType(t *testing.T) {
	_, typeMap := analyzeSpec(t, nullableUnionSpec)

	recipe := typeMap["Recipe"]
	if recipe == nil {
		t.Fatal("Recipe type not found")
	}
	fields := make(map[string]*ir.Field, len(recipe.Fields))
	for _, f := range recipe.Fields {
		fields[f.JSONName] = f
	}

	tests := []struct {
		field string
		want  string
	}{
		// Required, but nullable through the union, so still a pointer.
		{"name", "*string"},
		{"tags", "[]string"},
		{"cook", "*Person"},
	}
	for _, tt := range tests {
		f := fields[tt.field]
		if f == nil {
			t.Errorf("Recipe field %q not found", tt.field)
			continue
		}
		if f.Type != tt.want {
			t.Errorf("Recipe.%s type = %q, want %q", tt.field, f.Type, tt.want)
		}
	}
}

// TestUnion_InlineVariantsGetNamedTypes covers the rest of what issue #16 asked
// for: a union whose members are inline schemas used to degrade to a bare any,
// which callers could neither construct nor decode into.
func TestUnion_InlineVariantsGetNamedTypes(t *testing.T) {
	_, typeMap := analyzeSpec(t, `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Person:
      type: object
      properties:
        name: { type: string }
    Filter:
      type: object
      properties:
        value:
          anyOf: [{ type: string }, { type: array, items: { type: string } }, { type: "null" }]
          title: Value
        loc:
          type: array
          items:
            anyOf: [{ type: string }, { type: integer }]
        who:
          anyOf: [{ $ref: "#/components/schemas/Person" }, { type: string }]
`)

	filter := typeMap["Filter"]
	if filter == nil {
		t.Fatal("Filter type not found")
	}
	fields := make(map[string]*ir.Field, len(filter.Fields))
	for _, f := range filter.Fields {
		fields[f.JSONName] = f
	}

	for _, tt := range []struct{ field, want string }{
		{"value", "*Value"},
		{"loc", "[]FilterLocItem"},
		{"who", "*FilterWho"},
	} {
		f := fields[tt.field]
		if f == nil {
			t.Errorf("Filter field %q not found", tt.field)
			continue
		}
		if f.Type != tt.want {
			t.Errorf("Filter.%s type = %q, want %q", tt.field, f.Type, tt.want)
		}
	}

	// The `type: null` member says the union is nullable; it is not a shape the
	// value can take, so it gets no variant.
	value := typeMap["Value"]
	if value == nil {
		t.Fatal("synthesized union Value not found")
	}
	if value.Kind != ir.TypeKindUnion {
		t.Fatalf("Value kind = %v, want union", value.Kind)
	}
	got := make([]string, 0, len(value.UnionTypes))
	for _, v := range value.UnionTypes {
		got = append(got, v.TypeName)
	}
	if len(got) != 2 || got[0] != "string" || got[1] != "[]string" {
		t.Errorf("Value variants = %v, want [string []string]", got)
	}
}

// TestMultipartBody_BinaryPropertiesAreFiles checks that a binary property of a
// multipart body is generated as a file the caller can name, while the same
// format elsewhere stays a byte slice.
func TestMultipartBody_BinaryPropertiesAreFiles(t *testing.T) {
	_, typeMap := analyzeSpec(t, `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /image:
    put:
      operationId: putImage
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema: { $ref: "#/components/schemas/ImageUpload" }
      responses: { "204": { description: ok } }
  /doc:
    put:
      operationId: putDoc
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: "#/components/schemas/JSONUpload" }
      responses: { "204": { description: ok } }
components:
  schemas:
    ImageUpload:
      type: object
      required: [image]
      properties:
        image: { type: string, format: binary }
        thumbnail:
          anyOf: [{ type: string, format: binary }, { type: "null" }]
        attachments: { type: array, items: { type: string, format: binary } }
        extension: { type: string }
    JSONUpload:
      type: object
      required: [blob]
      properties:
        blob: { type: string, format: binary }
`)

	upload := typeMap["ImageUpload"]
	if upload == nil {
		t.Fatal("ImageUpload type not found")
	}
	fields := make(map[string]*ir.Field, len(upload.Fields))
	for _, f := range upload.Fields {
		fields[f.JSONName] = f
	}
	for _, tt := range []struct{ field, want string }{
		{"image", "FormFile"},
		{"thumbnail", "*FormFile"},
		{"attachments", "[]FormFile"},
		{"extension", "*string"},
	} {
		f := fields[tt.field]
		if f == nil {
			t.Errorf("ImageUpload field %q not found", tt.field)
			continue
		}
		if f.Type != tt.want {
			t.Errorf("ImageUpload.%s type = %q, want %q", tt.field, f.Type, tt.want)
		}
	}

	// Outside a multipart body, binary is still a byte slice.
	jsonUpload := typeMap["JSONUpload"]
	if jsonUpload == nil {
		t.Fatal("JSONUpload type not found")
	}
	if got := jsonUpload.Fields[0].Type; got != "[]byte" {
		t.Errorf("JSONUpload.blob type = %q, want []byte", got)
	}
}

// TestNullableUnion_SelfReferentialAliasesCompile pins that a schema whose only
// non-null member refers back to itself does not emit `type A = B; type B = A`,
// which Go rejects as an invalid recursive type.
func TestNullableUnion_SelfReferentialAliasesCompile(t *testing.T) {
	_, typeMap := analyzeSpec(t, `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Loop:
      anyOf: [{ $ref: "#/components/schemas/Loop" }, { type: "null" }]
    A:
      anyOf: [{ $ref: "#/components/schemas/B" }, { type: "null" }]
    B:
      anyOf: [{ $ref: "#/components/schemas/A" }, { type: "null" }]
    C:
      anyOf: [{ $ref: "#/components/schemas/D" }, { type: "null" }]
    D:
      anyOf: [{ type: array, items: { $ref: "#/components/schemas/C" } }, { type: "null" }]
`)

	// Follow every alias chain; none may return to a name already on it.
	for _, start := range []string{"Loop", "A", "B", "C", "D"} {
		td := typeMap[start]
		if td == nil {
			t.Errorf("%s type not found", start)
			continue
		}
		seen := map[string]bool{start: true}
		for td != nil && td.Kind == ir.TypeKindAlias {
			next := typeMap[aliasTarget(td.GoType)]
			if next == nil {
				break
			}
			if seen[next.Name] {
				t.Errorf("alias chain from %s cycles back to %s", start, next.Name)
				break
			}
			seen[next.Name] = true
			td = next
		}
	}
}

// TestUnion_AlongsideCompositionKeepsTheObject pins that a oneOf/anyOf used to
// constrain an object -- "exactly one of these is required", or a refinement of
// an allOf -- does not collapse the schema onto one of its members, throwing the
// declared properties and the composition away.
func TestUnion_AlongsideCompositionKeepsTheObject(t *testing.T) {
	_, typeMap := analyzeSpec(t, `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Wrapper:
      type: object
      properties: { w: { type: string } }
    ConstrainedObject:
      type: object
      properties:
        a: { type: string }
        b: { type: string }
      oneOf:
        - required: [a]
        - required: [b]
    ComposedWithUnion:
      allOf: [{ $ref: "#/components/schemas/Wrapper" }]
      oneOf: [{ type: string }, { type: string, format: date }]
`)

	obj := typeMap["ConstrainedObject"]
	if obj == nil || obj.Kind != ir.TypeKindStruct {
		t.Fatalf("ConstrainedObject = %+v, want a struct", obj)
	}
	if len(obj.Fields) != 2 {
		t.Errorf("ConstrainedObject has %d fields, want its two declared properties", len(obj.Fields))
	}

	composed := typeMap["ComposedWithUnion"]
	if composed == nil || composed.Kind != ir.TypeKindStruct {
		t.Fatalf("ComposedWithUnion = %+v, want a struct", composed)
	}
	if len(composed.Fields) != 1 || !composed.Fields[0].Embedded || composed.Fields[0].Type != "Wrapper" {
		t.Errorf("ComposedWithUnion fields = %+v, want the embedded Wrapper", composed.Fields)
	}
}
