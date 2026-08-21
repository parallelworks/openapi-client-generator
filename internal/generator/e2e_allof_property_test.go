package generator

import "testing"

const allOfPropertyAPISpec = `openapi: 3.1.0
info: { title: advisories, version: "1" }
paths:
  /advisories/{id}:
    get:
      operationId: getAdvisory
      parameters:
        - { name: id, in: path, required: true, schema: { type: string } }
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { $ref: "#/components/schemas/Advisory" } }
components:
  schemas:
    Advisory:
      type: object
      properties:
        id: { type: string }
        author:
          description: The author of the advisory.
          allOf:
            - $ref: "#/components/schemas/User"
            - type: "null"
        reviewer:
          description: Who reviewed it, with their verdict.
          allOf:
            - $ref: "#/components/schemas/User"
            - type: object
              properties:
                verdict: { type: string }
      required: [id]
    User:
      type: object
      properties:
        login: { type: string }
      required: [login]
`

// TestE2E_AllOfPropertiesAreTyped covers allOf where a property is typed. The
// analyzer composed it for a named schema and not for an inline one, so a
// property resolved to any and a caller had to assert their way into a schema
// the spec names.
func TestE2E_AllOfPropertiesAreTyped(t *testing.T) {
	files, _ := generateFromSpec(t, allOfPropertyAPISpec, "advapi")

	runGeneratedWireTest(t, files, "allofproperty", `package advapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrappedRefDecodesAsItsType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`+"`"+`{"id":"a-1","author":{"login":"ada"},"reviewer":{"login":"bo","verdict":"ok"}}`+"`"+`))
	}))
	defer srv.Close()

	adv, err := NewClient(srv.URL).GetAdvisory(t.Context(), "a-1")
	if err != nil {
		t.Fatalf("GetAdvisory: %v", err)
	}

	// A plain read rather than an assertion into map[string]any.
	if adv.Author == nil || adv.Author.Login != "ada" {
		t.Errorf("author = %+v, want the referenced User", adv.Author)
	}

	// A composition keeps both halves: what it references and what it adds.
	if adv.Reviewer == nil || adv.Reviewer.Login != "bo" {
		t.Errorf("reviewer = %+v, want the embedded User", adv.Reviewer)
	}
	if adv.Reviewer.Verdict == nil || *adv.Reviewer.Verdict != "ok" {
		t.Errorf("verdict = %v, want ok", adv.Reviewer.Verdict)
	}
}

// The pointer carries absence, so a null author stays distinguishable from one
// that is present and empty.
func TestNullWrappedRefIsNil(t *testing.T) {
	var adv Advisory
	if err := json.Unmarshal([]byte(`+"`"+`{"id":"a-2","author":null}`+"`"+`), &adv); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if adv.Author != nil {
		t.Errorf("author = %+v, want nil", adv.Author)
	}
}
`)
}
