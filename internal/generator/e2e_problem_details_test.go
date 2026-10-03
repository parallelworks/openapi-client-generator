package generator

import "testing"

const problemDetailsSpec = `openapi: "3.1.0"
info:
  title: Probs
  version: 1.0.0
paths:
  /widgets/{id}:
    get:
      operationId: getWidget
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Widget"
        default:
          description: unexpected error
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Failure"
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
        errors:
          type: array
          items:
            type: string
`

// TestE2E_ProblemDetails verifies that an RFC 9457 problem details body is
// read as one: Error() shows its detail (or title), Problem() exposes its
// members and extensions, and a typed error wrapper whose schema doesn't
// match the problem still shows the detail instead of the raw body.
func TestE2E_ProblemDetails(t *testing.T) {
	files, _ := generateFromSpec(t, problemDetailsSpec, "probs")

	runtimeTest := `package probs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const validation = ` + "`" + `{"type":"/problems/validation","title":"Invalid request","status":422,"detail":"Some fields aren't valid.","code":"validation","errors":[{"pointer":"#/name","detail":"Must be at most 8 characters."}]}` + "`" + `

func problemError(body string) *APIError {
	return &APIError{StatusCode: 422, Status: "422 Unprocessable Entity", ContentType: "application/problem+json; charset=utf-8", Body: []byte(body)}
}

func TestErrorShowsDetail(t *testing.T) {
	if got := problemError(validation).Error(); got != "API error 422 Unprocessable Entity: Some fields aren't valid." {
		t.Fatalf("Error() = %q", got)
	}
}

func TestErrorFallsBackToTitle(t *testing.T) {
	err := problemError(` + "`" + `{"type":"about:blank","title":"Not Found","status":404}` + "`" + `)
	if got := err.Error(); got != "API error 422 Unprocessable Entity: Not Found" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestProblemMembersAndExtensions(t *testing.T) {
	p := problemError(validation).Problem()
	if p == nil {
		t.Fatal("Problem() = nil")
	}
	if p.Type != "/problems/validation" || p.Title != "Invalid request" || p.Status != 422 || p.Detail != "Some fields aren't valid." {
		t.Errorf("members = %+v", p)
	}
	if string(p.Extensions["code"]) != ` + "`" + `"validation"` + "`" + ` || p.Extensions["errors"] == nil {
		t.Errorf("extensions = %v", p.Extensions)
	}
	if _, ok := p.Extensions["detail"]; ok {
		t.Error("a standard member is repeated in Extensions")
	}
}

func TestNotAProblemWithoutItsMediaType(t *testing.T) {
	err := &APIError{StatusCode: 400, Status: "400 Bad Request", ContentType: "application/json", Body: []byte(` + "`" + `{"detail":"x"}` + "`" + `)}
	if err.Problem() != nil {
		t.Error("Problem() parsed an application/json body")
	}
	if got := err.Error(); got != ` + "`" + `API error 400 Bad Request: {"detail":"x"}` + "`" + ` {
		t.Fatalf("Error() = %q", got)
	}
}

func TestMalformedProblemKeepsBody(t *testing.T) {
	err := problemError(` + "`" + `{"status":"bad"}` + "`" + `)
	if err.Problem() != nil {
		t.Error("Problem() accepted a status that isn't a number")
	}
	if got := err.Error(); got != ` + "`" + `API error 422 Unprocessable Entity: {"status":"bad"}` + "`" + ` {
		t.Fatalf("Error() = %q", got)
	}
}

// The typed wrapper's schema expects errors as strings; a problem's are
// objects, so it can't be parsed into it, and the problem's detail shows.
func TestWrapperThatDoesNotFitShowsDetail(t *testing.T) {
	err := parseFailureResponse(problemError(validation))
	if got := err.Error(); got != "API error 422 Unprocessable Entity: Some fields aren't valid." {
		t.Fatalf("Error() = %q", got)
	}
}

// A problem without the wrapper's message still parses into it; the wrapper
// then defers to the problem's detail.
func TestWrapperWithoutMessageShowsDetail(t *testing.T) {
	err := parseFailureResponse(problemError(` + "`" + `{"title":"Not Found","status":404,"detail":"There's no widget named a."}` + "`" + `))
	if got := err.Error(); got != "API error 422 Unprocessable Entity: There's no widget named a." {
		t.Fatalf("Error() = %q", got)
	}
}

func TestClientRecordsContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(` + "`" + `{"type":"about:blank","title":"Not Found","status":404,"detail":"There's no widget named a.","code":"not_found"}` + "`" + `))
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL).GetWidget(context.Background(), "a")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want an *APIError", err)
	}
	if p := apiErr.Problem(); p == nil || p.Detail != "There's no widget named a." || string(p.Extensions["code"]) != ` + "`" + `"not_found"` + "`" + ` {
		t.Fatalf("Problem() = %+v", p)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Error("errors.Is(err, ErrNotFound) = false")
	}
}
`
	runGeneratedWireTest(t, files, "probs", runtimeTest)
}
