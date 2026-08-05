package generator

import (
	"strings"
	"testing"
)

// TestE2E_RecursiveSchemasCompile covers the shapes a spec can use to define a
// type in terms of itself. Go allows that only through an indirection, so each
// of these used to generate an "invalid recursive type".
func TestE2E_RecursiveSchemasCompile(t *testing.T) {
	tests := []struct {
		name   string
		spec   string
		expect string
	}{
		{
			// A tree node: the most common recursive shape there is.
			name: "required self reference",
			spec: `
    Node:
      type: object
      required: [child, label]
      properties:
        label: { type: string }
        child: { $ref: "#/components/schemas/Node" }`,
			expect: "Child *Node `",
		},
		{
			name: "mutual reference",
			spec: `
    Parent:
      type: object
      required: [kid]
      properties:
        kid: { $ref: "#/components/schemas/Kid" }
    Kid:
      type: object
      required: [parent]
      properties:
        parent: { $ref: "#/components/schemas/Parent" }`,
			expect: "Parent *Parent `",
		},
		{
			// An alias between the two ends still closes the loop.
			name: "self reference through an alias",
			spec: `
    Wrapper:
      type: object
      required: [inner]
      properties:
        inner: { $ref: "#/components/schemas/AliasToWrapper" }
    AliasToWrapper:
      anyOf: [{ $ref: "#/components/schemas/Wrapper" }, { type: "null" }]`,
			expect: "Inner *AliasToWrapper `",
		},
		{
			// A slice already breaks the recursion, so nothing should change.
			name: "self reference through a slice stays a value",
			spec: `
    Branch:
      type: object
      required: [children]
      properties:
        children:
          type: array
          items: { $ref: "#/components/schemas/Branch" }`,
			expect: "Children []Branch `",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			build, files := generateAndBuild(t, `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:`+tt.spec+"\n")
			if build != "" {
				t.Fatalf("generated client does not compile:\n%s", build)
			}
			if types := files["types.go"]; !strings.Contains(types, tt.expect) {
				t.Errorf("types.go missing %q:\n%s", tt.expect, types)
			}
		})
	}
}

// TestE2E_RequestBodyWithoutSchemaCompiles covers a body the spec declares
// without saying what goes in it. The operation still needs a parameter type, or
// the method signature is a syntax error.
func TestE2E_RequestBodyWithoutSchemaCompiles(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		expect string
	}{
		{
			name:   "no schema under a raw media type",
			body:   "        content:\n          application/xml: {}",
			expect: "func (c *Client) Send(ctx context.Context, body []byte) error",
		},
		{
			name:   "no schema under a text media type",
			body:   "        content:\n          text/plain: {}",
			expect: "func (c *Client) Send(ctx context.Context, body string) error",
		},
		{
			name:   "no schema under json",
			body:   "        content:\n          application/json: {}",
			expect: "func (c *Client) Send(ctx context.Context, body any) error",
		},
		{
			// Nothing to send at all, so the method takes no body.
			name:   "no content at all",
			body:   "        description: nothing",
			expect: "func (c *Client) Send(ctx context.Context) error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			build, files := generateAndBuild(t, `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /s:
    post:
      operationId: send
      requestBody:
        required: true
`+tt.body+`
      responses: { "204": { description: ok } }
`)
			if build != "" {
				t.Fatalf("generated client does not compile:\n%s", build)
			}
			ops := files["operations.go"]
			if !strings.Contains(ops, tt.expect) {
				t.Errorf("operations.go missing %q:\n%s", tt.expect, ops)
			}
		})
	}
}
