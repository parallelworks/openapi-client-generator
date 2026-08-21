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

// specPrefix marks every name this spec supplies, so what the templates declare
// on their own is whatever does not carry it.
const specPrefix = "Zqx"

const reservedProbeSpec = `openapi: 3.1.0
info: { title: zqx, version: "1" }
servers:
  - url: https://{zqxregion}.example.com
    variables:
      zqxregion: { default: one }
webhooks:
  zqxHook:
    post:
      requestBody:
        content:
          application/json: { schema: { $ref: "#/components/schemas/ZqxThing" } }
paths:
  /things:
    get:
      operationId: zqxList
      parameters:
        - { name: cursor, in: query, schema: { type: string } }
      responses:
        "200":
          description: ok
          headers:
            X-Zqx-Trace: { schema: { type: string } }
          content:
            application/json: { schema: { $ref: "#/components/schemas/ZqxPage" } }
        default:
          description: err
          content:
            application/json: { schema: { $ref: "#/components/schemas/ZqxFailure" } }
    post:
      operationId: zqxUpload
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file: { type: string, format: binary }
      responses:
        "204": { description: ok }
      callbacks:
        zqxBack:
          "{$request.body#/url}":
            post:
              requestBody:
                content:
                  application/json: { schema: { $ref: "#/components/schemas/ZqxThing" } }
components:
  schemas:
    ZqxThing:
      type: object
      properties:
        name: { type: string }
    ZqxPage:
      type: object
      properties:
        items: { type: array, items: { $ref: "#/components/schemas/ZqxThing" } }
        nextCursor: { type: string }
    ZqxFailure:
      type: object
      properties:
        message: { type: string }
`

// TestReservedIdentifiersCoverTheTemplates fails when a template starts
// declaring a package-level name the list does not hold. Without it the list is
// kept in sync by hand, and a missing entry is invisible until a spec happens to
// use that name, at which point the generated package does not compile.
func TestReservedIdentifiersCoverTheTemplates(t *testing.T) {
	files, _ := generateFromSpec(t, reservedProbeSpec, "zqxapi")

	fset := token.NewFileSet()
	var missing []string
	for _, f := range files {
		parsed, err := parser.ParseFile(fset, f.Name, f.Content, 0)
		if err != nil {
			t.Fatalf("parsing generated %s: %v", f.Name, err)
		}
		for _, name := range exportedDecls(parsed) {
			// Names the spec supplied are renamed by the naming scope when they
			// collide, so they are not the list's business.
			if strings.Contains(name, specPrefix) {
				continue
			}
			if !slices.Contains(templates.ReservedIdentifiers, name) && !slices.Contains(missing, name) {
				missing = append(missing, name)
			}
		}
	}

	if len(missing) > 0 {
		t.Errorf("templates declare %v, which ReservedIdentifiers does not hold: a schema of that name would redeclare it", missing)
	}
}

// exportedDecls returns the exported package-level names a file declares.
// Methods are left out: they live in their receiver's method set rather than the
// package scope, so a schema cannot collide with one.
func exportedDecls(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && d.Name.IsExported() {
				names = append(names, d.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.IsExported() {
						names = append(names, s.Name.Name)
					}
				case *ast.ValueSpec:
					for _, ident := range s.Names {
						if ident.IsExported() {
							names = append(names, ident.Name)
						}
					}
				}
			}
		}
	}
	return names
}
