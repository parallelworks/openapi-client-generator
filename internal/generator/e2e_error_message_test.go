package generator

import "testing"

const errorMessageSpec = `openapi: "3.1.0"
info:
  title: Errmsg
  version: 1.0.0
paths:
  /widgets:
    get:
      operationId: listWidgets
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Widget"
        default:
          description: unexpected error
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Failure"
  /gadgets:
    get:
      operationId: listGadgets
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Widget"
        default:
          description: unexpected error
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Problem"
  /gizmos:
    get:
      operationId: listGizmos
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Widget"
        default:
          description: unexpected error
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Bare"
  /doodads:
    get:
      operationId: listDoodads
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/Widget"
        default:
          description: unexpected error
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Opaque"
components:
  schemas:
    Widget:
      type: object
      properties:
        id:
          type: string
    Failure:
      type: object
      required:
        - message
      properties:
        message:
          type: string
          x-ms-primary-error-message: true
    Problem:
      type: object
      properties:
        title:
          type: string
        detail:
          type: string
          x-ms-primary-error-message: true
    Bare:
      type: object
      properties:
        message:
          type: string
    Opaque:
      type: object
      properties:
        code:
          type: integer
        trace:
          type: object
          properties:
            span:
              type: string
`

// TestE2E_ErrorResponseMessage verifies a parsed error response surfaces the
// field annotated x-ms-primary-error-message instead of dumping the raw body,
// for required (value) and optional (pointer) fields, and that unannotated
// schemas keep the raw-body output.
func TestE2E_ErrorResponseMessage(t *testing.T) {
	files, _ := generateFromSpec(t, errorMessageSpec, "errmsg")

	runtimeTest := `package errmsg

import (
	"strings"
	"testing"
)

func TestAnnotatedMessageRendered(t *testing.T) {
	err := parseFailureResponse(&APIError{StatusCode: 404, Status: "404 Not Found", Body: []byte(` + "`" + `{"message":"Widget not found"}` + "`" + `)})
	if got := err.Error(); got != "API error 404 Not Found: Widget not found" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestAnnotatedPointerFieldRendered(t *testing.T) {
	err := parseProblemResponse(&APIError{StatusCode: 403, Status: "403 Forbidden", Body: []byte(` + "`" + `{"title":"Forbidden","detail":"Access denied"}` + "`" + `)})
	if got := err.Error(); got != "API error 403 Forbidden: Access denied" {
		t.Fatalf("Error() = %q", got)
	}
}

// A schema that marks nothing still names its message the way most APIs do.
func TestConventionalNameIsRendered(t *testing.T) {
	err := parseBareResponse(&APIError{StatusCode: 404, Status: "404 Not Found", Body: []byte(` + "`" + `{"message":"Widget not found"}` + "`" + `)})
	if got := err.Error(); got != "API error 404 Not Found: Widget not found" {
		t.Fatalf("Error() = %q", got)
	}
}

// A body with no property that reads as a message keeps the raw output, which is
// the only honest thing to print.
func TestBodyWithoutAMessageKeepsBody(t *testing.T) {
	err := parseOpaqueResponse(&APIError{StatusCode: 500, Status: "500 Internal Server Error", Body: []byte(` + "`" + `{"code":7}` + "`" + `)})
	if got := err.Error(); !strings.Contains(got, ` + "`" + `{"code":7}` + "`" + `) {
		t.Fatalf("Error() = %q", got)
	}
}

func TestEmptyMessageFallsBackToBody(t *testing.T) {
	err := parseFailureResponse(&APIError{StatusCode: 500, Status: "500 Internal Server Error", Body: []byte(` + "`" + `{"message":""}` + "`" + `)})
	if got := err.Error(); !strings.Contains(got, ` + "`" + `{"message":""}` + "`" + `) {
		t.Fatalf("Error() = %q", got)
	}
}

func TestUnparseableBodyFallsBackToBody(t *testing.T) {
	err := parseFailureResponse(&APIError{StatusCode: 502, Status: "502 Bad Gateway", Body: []byte("upstream exploded")})
	if got := err.Error(); got != "API error 502 Bad Gateway: upstream exploded" {
		t.Fatalf("Error() = %q", got)
	}
}
`
	runGeneratedWireTest(t, files, "errmsg", runtimeTest)
}
