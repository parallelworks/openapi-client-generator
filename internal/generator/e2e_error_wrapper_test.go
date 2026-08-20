package generator

import "testing"

const unnamedErrorBodySpec = `openapi: 3.1.0
info: { title: errbody, version: "1" }
paths:
  /t:
    get:
      operationId: getT
      responses:
        "200": { description: ok, content: { application/json: { schema: { type: string } } } }
        "404":
          description: missing
          content:
            application/json:
              schema: { type: array, items: { type: string } }
        default:
          description: fallback
          content:
            application/json:
              schema: { type: object }
  /u:
    get:
      operationId: getU
      responses:
        "200": { description: ok, content: { application/json: { schema: { type: string } } } }
        "422":
          description: invalid
          content:
            application/json:
              schema: { $ref: "#/components/schemas/ValidationErrors" }
        "500":
          description: oops
          content:
            text/plain:
              schema: { type: string }
components:
  schemas:
    ValidationErrors:
      type: array
      items: { $ref: "#/components/schemas/ValidationError" }
    ValidationError:
      type: object
      properties:
        field: { type: string }
        message: { type: string }
`

// TestE2E_UnnamedErrorBodyCompiles covers error bodies whose Go type is an
// expression rather than a name: a map, a slice, and a builtin. Pasting the type
// into the wrapper's declaration put `type map[string]anyResponse` in the output,
// which is not Go.
func TestE2E_UnnamedErrorBodyCompiles(t *testing.T) {
	files, _ := generateFromSpec(t, unnamedErrorBodySpec, "errbody")

	runGeneratedWireTest(t, files, "errbody", `package errbody

import (
	"errors"
	"testing"
)

func TestMapBodyParsesIntoTheWrapper(t *testing.T) {
	err := parseGetTDefaultResponseError(&APIError{StatusCode: 500, Status: "500 Internal Server Error", Body: []byte(`+"`"+`{"reason":"upstream"}`+"`"+`)})

	var wrapped *GetTDefaultResponseError
	if !errors.As(err, &wrapped) {
		t.Fatalf("errors.As did not match the wrapper: %T", err)
	}
	if wrapped.Detail["reason"] != "upstream" {
		t.Errorf("Detail = %v, want the parsed body", wrapped.Detail)
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 500 {
		t.Error("the wrapper stopped unwrapping to APIError")
	}
}

func TestSliceBodyParsesIntoTheWrapper(t *testing.T) {
	err := parseGetTResponse404Error(&APIError{StatusCode: 404, Status: "404 Not Found", Body: []byte(`+"`"+`["gone","really gone"]`+"`"+`)})

	var wrapped *GetTResponse404Error
	if !errors.As(err, &wrapped) {
		t.Fatalf("errors.As did not match the wrapper: %T", err)
	}
	if len(wrapped.Detail) != 2 || wrapped.Detail[0] != "gone" {
		t.Errorf("Detail = %v, want the parsed array", wrapped.Detail)
	}
}

// A body whose Go type is a builtin has no name to build an identifier from
// either, and the wrapper it lands in has to stay exported for errors.As.
func TestBuiltinBodyWrapperIsExported(t *testing.T) {
	err := parseGetUResponse500Error(&APIError{StatusCode: 500, Status: "500 Internal Server Error", Body: []byte(`+"`"+`"plain text"`+"`"+`)})

	var wrapped *GetUResponse500Error
	if !errors.As(err, &wrapped) {
		t.Fatalf("errors.As did not match the wrapper: %T", err)
	}
	if wrapped.Detail != "plain text" {
		t.Errorf("Detail = %q, want the parsed body", wrapped.Detail)
	}
}

// A named body keeps the wrapper name it already had, so upgrading does not
// rename types callers match on.
func TestNamedBodyKeepsItsWrapperName(t *testing.T) {
	err := parseValidationErrorsResponse(&APIError{StatusCode: 422, Status: "422 Unprocessable Entity", Body: []byte(`+"`"+`[{"field":"name","message":"required"}]`+"`"+`)})

	var wrapped *ValidationErrorsResponse
	if !errors.As(err, &wrapped) {
		t.Fatalf("errors.As did not match the wrapper: %T", err)
	}
	if len(wrapped.Detail) != 1 || wrapped.Detail[0].Field == nil || *wrapped.Detail[0].Field != "name" {
		t.Errorf("Detail = %v, want the parsed body", wrapped.Detail)
	}
}
`)
}
