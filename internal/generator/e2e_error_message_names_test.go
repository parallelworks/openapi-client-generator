package generator

import "testing"

const messageNameSpec = `openapi: 3.1.0
info: { title: errs, version: "1" }
paths:
  /a:
    get:
      operationId: getA
      responses:
        "200": { description: ok, content: { application/json: { schema: { type: string } } } }
        default:
          description: err
          content:
            application/json: { schema: { $ref: "#/components/schemas/Problem" } }
  /b:
    get:
      operationId: getB
      responses:
        "200": { description: ok, content: { application/json: { schema: { type: string } } } }
        default:
          description: err
          content:
            application/json: { schema: { $ref: "#/components/schemas/Oauth" } }
  /c:
    get:
      operationId: getC
      responses:
        "200": { description: ok, content: { application/json: { schema: { type: string } } } }
        default:
          description: err
          content:
            application/json: { schema: { $ref: "#/components/schemas/Marked" } }
  /d:
    get:
      operationId: getD
      responses:
        "200": { description: ok, content: { application/json: { schema: { type: string } } } }
        default:
          description: err
          content:
            application/json: { schema: { $ref: "#/components/schemas/Typed" } }
components:
  schemas:
    Problem:
      type: object
      properties:
        type: { type: string }
        title: { type: string }
        detail: { type: string }
        status: { type: integer }
    Oauth:
      type: object
      properties:
        error: { type: string }
        error_description: { type: string }
    Marked:
      type: object
      properties:
        message: { type: string }
        note:
          type: string
          x-error-message: true
    Typed:
      type: object
      properties:
        message: { type: integer }
        detail: { type: string }
`

// TestE2E_ErrorMessageNames covers the error bodies that mark nothing, which is
// most of them: Error() printed the whole JSON body rather than the property
// that carries the message.
func TestE2E_ErrorMessageNames(t *testing.T) {
	files, _ := generateFromSpec(t, messageNameSpec, "errsapi")

	runGeneratedWireTest(t, files, "messagenames", `package errsapi

import (
	"strings"
	"testing"
)

func rendered(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	return err.Error()
}

// RFC 7807 carries both title and detail, and detail is the specific one.
func TestProblemPrefersDetail(t *testing.T) {
	err := parseProblemResponse(&APIError{StatusCode: 400, Status: "400 Bad Request", Body: []byte(`+"`"+`{"title":"Bad Request","detail":"name is required"}`+"`"+`)})
	if got := rendered(t, err); got != "API error 400 Bad Request: name is required" {
		t.Fatalf("Error() = %q", got)
	}
}

// With only title, that is the message.
func TestProblemFallsBackToTitle(t *testing.T) {
	err := parseProblemResponse(&APIError{StatusCode: 400, Status: "400 Bad Request", Body: []byte(`+"`"+`{"title":"Bad Request"}`+"`"+`)})
	if got := rendered(t, err); got != "API error 400 Bad Request: Bad Request" {
		t.Fatalf("Error() = %q", got)
	}
}

// OAuth error bodies name the human-readable half error_description, with error
// holding a code.
func TestOAuthPrefersDescription(t *testing.T) {
	err := parseOauthResponse(&APIError{StatusCode: 401, Status: "401 Unauthorized", Body: []byte(`+"`"+`{"error":"invalid_grant","error_description":"Token expired"}`+"`"+`)})
	if got := rendered(t, err); got != "API error 401 Unauthorized: Token expired" {
		t.Fatalf("Error() = %q", got)
	}
}

// A marked property wins over a conventionally named one, whichever vocabulary
// marked it.
func TestMarkedPropertyWins(t *testing.T) {
	err := parseMarkedResponse(&APIError{StatusCode: 500, Status: "500 Internal Server Error", Body: []byte(`+"`"+`{"message":"generic","note":"the real one"}`+"`"+`)})
	if got := rendered(t, err); got != "API error 500 Internal Server Error: the real one" {
		t.Fatalf("Error() = %q", got)
	}
}

// A conventional name holding something other than text is not the message.
func TestNonStringNameIsSkipped(t *testing.T) {
	err := parseTypedResponse(&APIError{StatusCode: 500, Status: "500 Internal Server Error", Body: []byte(`+"`"+`{"message":7,"detail":"the real one"}`+"`"+`)})
	if got := rendered(t, err); got != "API error 500 Internal Server Error: the real one" {
		t.Fatalf("Error() = %q", got)
	}
}

// An empty message still falls back to the body, so nothing disappears.
func TestEmptyConventionalMessageFallsBack(t *testing.T) {
	err := parseProblemResponse(&APIError{StatusCode: 400, Status: "400 Bad Request", Body: []byte(`+"`"+`{"detail":""}`+"`"+`)})
	if got := rendered(t, err); !strings.Contains(got, `+"`"+`{"detail":""}`+"`"+`) {
		t.Fatalf("Error() = %q", got)
	}
}
`)
}
