package generator

import "testing"

const operationSecurityAPISpec = `openapi: 3.1.0
info: { title: sec, version: "1" }
security:
  - bearer: []
paths:
  /private:
    get:
      operationId: getPrivate
      responses: { "204": { description: ok } }
  /public:
    get:
      operationId: getPublic
      security: []
      responses: { "204": { description: ok } }
  /public-body:
    post:
      operationId: postPublic
      security: []
      requestBody:
        required: true
        content:
          application/json: { schema: { type: string } }
      responses: { "204": { description: ok } }
components:
  securitySchemes:
    bearer: { type: http, scheme: bearer }
`

// TestE2E_OperationSecurityOptOut covers an operation declaring security: [].
// Auth was applied for every request the client made, so an endpoint the spec
// documents as needing no credential received one anyway.
func TestE2E_OperationSecurityOptOut(t *testing.T) {
	files, _ := generateFromSpec(t, operationSecurityAPISpec, "secapi")

	runGeneratedWireTest(t, files, "operationsecurity", `package secapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func authSeen(t *testing.T) (*Client, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, WithAuth(&BearerAuth{Token: "s3cr3t"})), &seen
}

func TestPublicOperationSendsNoCredential(t *testing.T) {
	client, seen := authSeen(t)

	if err := client.GetPrivate(t.Context()); err != nil {
		t.Fatalf("GetPrivate: %v", err)
	}
	if err := client.GetPublic(t.Context()); err != nil {
		t.Fatalf("GetPublic: %v", err)
	}

	if (*seen)[0] != "Bearer s3cr3t" {
		t.Errorf("the operation inheriting the document's security got %q", (*seen)[0])
	}
	if (*seen)[1] != "" {
		t.Errorf("the operation declaring security: [] got %q", (*seen)[1])
	}
}

// Opting out of auth changes nothing else about the request.
func TestPublicOperationStillSendsItsBody(t *testing.T) {
	var payload, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		r.Body.Read(buf)
		payload, auth = string(buf), r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, WithAuth(&BearerAuth{Token: "s3cr3t"}))
	if err := client.PostPublic(t.Context(), "hello"); err != nil {
		t.Fatalf("PostPublic: %v", err)
	}
	if payload != `+"`"+`"hello"`+"`"+` {
		t.Errorf("payload = %q", payload)
	}
	if auth != "" {
		t.Errorf("Authorization = %q, want none", auth)
	}
}

// A client with no provider is unaffected either way.
func TestNoProviderIsUnaffected(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := NewClient(srv.URL).GetPrivate(t.Context()); err != nil {
		t.Fatalf("GetPrivate: %v", err)
	}
	if seen[0] != "" {
		t.Errorf("Authorization = %q, want none", seen[0])
	}
}
`)
}
