package analyzer

import "testing"

const allOfPropertySpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    View:
      type: object
      properties:
        creator:
          allOf:
            - $ref: "#/components/schemas/User"
        author:
          readOnly: true
          description: The author.
          allOf:
            - $ref: "#/components/schemas/User"
        same:
          allOf:
            - $ref: "#/components/schemas/User"
        composed:
          description: A user with a role beside it.
          allOf:
            - $ref: "#/components/schemas/User"
            - type: object
              properties:
                role: { type: string }
        inline:
          allOf:
            - type: object
              properties:
                only: { type: string }
    User:
      type: object
      properties:
        login: { type: string }
      required: [login]
`

// allOf is JSON Schema composition, and a lone $ref inside one says the value
// must match that schema, which is to say it is of that type.
func TestAllOfProperty_LoneRefResolvesToIt(t *testing.T) {
	_, typeMap := analyzeSpec(t, allOfPropertySpec)

	view := typeMap["View"]
	if view == nil {
		t.Fatal("View not found")
	}
	byJSON := map[string]string{}
	for _, f := range view.Fields {
		byJSON[f.JSONName] = f.Type
	}

	if byJSON["creator"] != "*User" {
		t.Errorf("creator = %q, want *User", byJSON["creator"])
	}
	// Keywords beside the allOf are handled where they belong, so the
	// composition still resolves to what it composes.
	if byJSON["author"] != "*User" {
		t.Errorf("author = %q, want *User", byJSON["author"])
	}
	if byJSON["same"] != "*User" {
		t.Errorf("same = %q, want *User", byJSON["same"])
	}
}

// A composition of more than a reference is a shape of its own, and gets a name
// like any other inline schema.
func TestAllOfProperty_CompositionIsNamed(t *testing.T) {
	_, typeMap := analyzeSpec(t, allOfPropertySpec)

	view := typeMap["View"]
	byJSON := map[string]string{}
	for _, f := range view.Fields {
		byJSON[f.JSONName] = f.Type
	}

	if byJSON["composed"] != "*ViewComposed" {
		t.Fatalf("composed = %q, want *ViewComposed", byJSON["composed"])
	}
	composed := typeMap["ViewComposed"]
	if composed == nil {
		t.Fatal("ViewComposed not synthesized")
	}
	if len(composed.Fields) != 2 {
		t.Fatalf("ViewComposed fields = %+v, want the embed and the property", composed.Fields)
	}
	if !composed.Fields[0].Embedded || composed.Fields[0].Type != "User" {
		t.Errorf("ViewComposed field 0 = %+v, want an embedded User", composed.Fields[0])
	}
	if composed.Fields[1].JSONName != "role" {
		t.Errorf("ViewComposed field 1 = %+v, want role", composed.Fields[1])
	}

	// A single inline entry composes just as much as several do.
	if byJSON["inline"] != "*ViewInline" {
		t.Errorf("inline = %q, want *ViewInline", byJSON["inline"])
	}
}

const refIdiomSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Advisory:
      type: object
      properties:
        siblings:
          $ref: "#/components/schemas/User"
          description: 3.1 allows keywords beside a reference.
          readOnly: true
        nullableRef:
          anyOf:
            - $ref: "#/components/schemas/User"
            - type: "null"
    User:
      type: object
      properties: { login: { type: string } }
      required: [login]
`

// The ways 3.1 says the same things without allOf: keywords beside a $ref, and a
// null member for a nullable one. Both resolve to the referenced type, and this
// keeps them that way.
func TestRefIdioms_3_1(t *testing.T) {
	_, typeMap := analyzeSpec(t, refIdiomSpec)

	byJSON := map[string]string{}
	for _, f := range typeMap["Advisory"].Fields {
		byJSON[f.JSONName] = f.Type
	}
	if byJSON["siblings"] != "*User" {
		t.Errorf("a $ref with sibling keywords = %q, want *User", byJSON["siblings"])
	}
	if byJSON["nullableRef"] != "*User" {
		t.Errorf("anyOf with a null member = %q, want *User", byJSON["nullableRef"])
	}
}

const allOfMultipartSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /upload:
    post:
      operationId: upload
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              allOf:
                - $ref: "#/components/schemas/Meta"
                - type: object
                  properties:
                    file: { type: string, format: binary }
                  required: [file]
      responses:
        "204": { description: ok }
components:
  schemas:
    Meta:
      type: object
      properties:
        label: { type: string }
      required: [label]
`

// Multipart is a property of where a schema is used, and a body composed through
// allOf is used the same way one written as an object is: its binary properties
// are file parts rather than base64 text in an ordinary field.
func TestAllOfProperty_MultipartBodyKeepsItsFileParts(t *testing.T) {
	_, typeMap := analyzeSpec(t, allOfMultipartSpec)

	body := typeMap["UploadBody"]
	if body == nil {
		t.Fatal("UploadBody not found")
	}
	byJSON := map[string]string{}
	for _, f := range body.Fields {
		byJSON[f.JSONName] = f.Type
	}
	if byJSON["file"] != "FormFile" {
		t.Errorf("file = %q, want FormFile", byJSON["file"])
	}
	// What it composes comes along.
	if len(body.Fields) != 2 || !body.Fields[0].Embedded || body.Fields[0].Type != "Meta" {
		t.Errorf("fields = %+v, want the embedded Meta beside the file", body.Fields)
	}
}
