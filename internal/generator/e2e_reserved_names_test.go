package generator

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/templates"
)

// reservedNamesSpec declares no operations and no error bodies, so every
// exported package-level name in the output but types.go is one the templates
// always declare, with nothing derived mixed in.
const reservedNamesSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  securitySchemes:
    bearer: { type: http, scheme: bearer }
    apiKey: { type: apiKey, name: X-Key, in: header }
    basic: { type: http, scheme: basic }
  schemas:
    Thing:
      type: object
      properties:
        name: { type: string }
`

// TestReservedIdentifiersCoversWhatTheTemplatesDeclare parses the generated
// files and asserts every exported package-level name is reserved. Asserting
// against templates.ReservedIdentifiers by iterating it can only confirm the
// entries already there; this asks the output what it actually declares, so a
// name added to a template without being reserved fails here.
func TestReservedIdentifiersCoversWhatTheTemplatesDeclare(t *testing.T) {
	_, files := generateAndBuild(t, reservedNamesSpec)

	fset := token.NewFileSet()
	for name, src := range files {
		// types.go holds the schemas themselves, which are meant to be spec-named.
		if name == "types.go" {
			continue
		}
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatalf("parsing generated %s: %v", name, err)
		}
		for _, declared := range exportedPackageNames(file) {
			if !slices.Contains(templates.ReservedIdentifiers, declared) {
				t.Errorf("%s declares %q at package scope but it is not in templates.ReservedIdentifiers, "+
					"so a schema of that name would redeclare it", name, declared)
			}
		}
	}
}

// exportedPackageNames returns the exported types, funcs, vars, and consts a
// file declares at package scope. Methods take no package-scope name.
func exportedPackageNames(file *ast.File) []string {
	var names []string
	add := func(name string) {
		if ast.IsExported(name) {
			names = append(names, name)
		}
	}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				add(d.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					add(s.Name.Name)
				case *ast.ValueSpec:
					for _, ident := range s.Names {
						add(ident.Name)
					}
				}
			}
		}
	}
	return names
}

// TestE2E_SchemaNamedLikeADerivedType covers the identifiers the templates build
// at render time rather than always declaring: a params struct is named after
// its operation and an error wrapper after the body it wraps, so neither can sit
// in a static list.
func TestE2E_SchemaNamedLikeADerivedType(t *testing.T) {
	build, files := generateAndBuild(t, `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /u:
    get:
      operationId: listUsers
      parameters: [{ name: q, in: query, schema: { type: string } }]
      responses:
        "200": { description: ok }
        "404":
          description: nf
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Error" }
components:
  schemas:
    Error:
      type: object
      properties: { message: { type: string } }
    ErrorResponse:
      type: object
      properties: { y: { type: string } }
    ListUsersParams:
      type: object
      properties: { z: { type: string } }
`)
	if build != "" {
		t.Fatalf("generated client does not compile:\n%s", build)
	}
	types := files["types.go"]
	for _, want := range []string{"type ErrorResponse2 struct", "type ListUsersParams2 struct"} {
		if !strings.Contains(types, want) {
			t.Errorf("types.go missing %q — the schema was not renamed off the derived name:\n%s", want, types)
		}
	}
}

// TestE2E_TraceOperationIsGenerated covers the one HTTP method the path-item
// walk used to skip.
func TestE2E_TraceOperationIsGenerated(t *testing.T) {
	build, files := generateAndBuild(t, `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /u:
    trace:
      operationId: traceUsers
      responses: { "204": { description: ok } }
`)
	if build != "" {
		t.Fatalf("generated client does not compile:\n%s", build)
	}
	if ops := files["operations.go"]; !strings.Contains(ops, "func (c *Client) TraceUsers(") {
		t.Errorf("operations.go has no method for the trace operation:\n%s", ops)
	}
}

// TestE2E_FormBodyIsAlwaysEncodable covers a form body whose schema is not an
// object: the encoders walk properties, so a scalar would compile and then fail
// on every call.
func TestE2E_FormBodyIsAlwaysEncodable(t *testing.T) {
	for _, tt := range []struct{ name, contentType string }{
		{"urlencoded", "application/x-www-form-urlencoded"},
		{"multipart", "multipart/form-data"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			build, files := generateAndBuild(t, `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /f:
    post:
      operationId: postForm
      requestBody:
        content:
          `+tt.contentType+`:
            schema: { type: string }
      responses: { "204": { description: ok } }
`)
			if build != "" {
				t.Fatalf("generated client does not compile:\n%s", build)
			}
			ops := files["operations.go"]
			if !strings.Contains(ops, "body *map[string]any") {
				t.Errorf("a %s body that is not an object should still take something the encoder accepts:\n%s", tt.contentType, ops)
			}
		})
	}
}
