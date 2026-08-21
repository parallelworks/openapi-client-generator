package generator

import "testing"

const optionalBodySpec = `openapi: 3.1.0
info: { title: opt, version: "1" }
paths:
  /things:
    post:
      operationId: createThing
      requestBody:
        required: false
        content:
          application/json: { schema: { $ref: "#/components/schemas/Thing" } }
      responses: { "204": { description: ok } }
  /required:
    post:
      operationId: createRequired
      requestBody:
        required: true
        content:
          application/json: { schema: { $ref: "#/components/schemas/Thing" } }
      responses: { "204": { description: ok } }
components:
  schemas:
    Thing:
      type: object
      properties:
        name: { type: string }
`

// TestE2E_OptionalBodyOmitted covers an optional body left out. It reached the
// encoder as a nil pointer inside an interface, which is not nil, so the request
// carried the JSON literal null and a Content-Type to go with it.
func TestE2E_OptionalBodyOmitted(t *testing.T) {
	files, _ := generateFromSpec(t, optionalBodySpec, "optapi")

	runGeneratedWireTest(t, files, "optionalbody", `package optapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func sent(t *testing.T) (*Client, *string, *string, *int64) {
	t.Helper()
	var payload, ctype string
	var length int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		payload, ctype, length = string(b), r.Header.Get("Content-Type"), r.ContentLength
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL), &payload, &ctype, &length
}

func TestNilOptionalBodySendsNothing(t *testing.T) {
	client, payload, ctype, length := sent(t)

	if err := client.CreateThing(t.Context(), nil); err != nil {
		t.Fatalf("CreateThing: %v", err)
	}
	if *payload != "" {
		t.Errorf("payload = %q, want no body at all", *payload)
	}
	if *ctype != "" {
		t.Errorf("Content-Type = %q, want none for a request with no body", *ctype)
	}
	if *length > 0 {
		t.Errorf("Content-Length = %d, want no body", *length)
	}
}

// A body that is present still goes, including one whose fields are all unset.
func TestPresentOptionalBodyIsSent(t *testing.T) {
	client, payload, ctype, _ := sent(t)

	if err := client.CreateThing(t.Context(), &Thing{}); err != nil {
		t.Fatalf("CreateThing: %v", err)
	}
	if *payload != "{}" {
		t.Errorf("payload = %q, want the empty object the caller passed", *payload)
	}
	if *ctype != "application/json" {
		t.Errorf("Content-Type = %q", *ctype)
	}
}

func TestRequiredBodyIsUnaffected(t *testing.T) {
	client, payload, _, _ := sent(t)

	name := "a"
	if err := client.CreateRequired(t.Context(), Thing{Name: &name}); err != nil {
		t.Fatalf("CreateRequired: %v", err)
	}
	if *payload != `+"`"+`{"name":"a"}`+"`"+` {
		t.Errorf("payload = %q", *payload)
	}
}
`)
}
