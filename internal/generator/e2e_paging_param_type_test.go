package generator

import (
	"strings"
	"testing"
)

const stringPageSpec = `openapi: 3.1.0
info: { title: search, version: "1" }
paths:
  /search:
    get:
      operationId: search
      parameters:
        - { name: page, in: query, schema: { type: string, maxLength: 5000 } }
        - { name: limit, in: query, schema: { type: integer } }
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { $ref: "#/components/schemas/Page" } }
  /list:
    get:
      operationId: list
      parameters:
        - { name: page, in: query, schema: { type: integer } }
        - { name: limit, in: query, schema: { type: integer } }
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { $ref: "#/components/schemas/Page" } }
components:
  schemas:
    Page:
      type: object
      properties:
        items: { type: array, items: { $ref: "#/components/schemas/Item" } }
    Item:
      type: object
      properties:
        id: { type: string }
`

// TestE2E_StringPageParamStillCompiles covers a page parameter that carries a
// token rather than a count, which Stripe's search endpoints declare. The
// iterator converts its position through int64, so generating one produced a
// package that did not build.
func TestE2E_StringPageParamStillCompiles(t *testing.T) {
	files, _ := generateFromSpec(t, stringPageSpec, "searchapi")

	var pagination string
	for _, f := range files {
		if f.Name == "pagination.go" {
			pagination = string(f.Content)
		}
	}
	if strings.Contains(pagination, "SearchIter") {
		t.Error("an operation whose page is a string should get no iterator")
	}
	if !strings.Contains(pagination, "ListIter") {
		t.Error("an operation whose page is a number should still get one")
	}

	// The point of the test: the package builds.
	buildGenerated(t, files, "stringpage")
}
