package generator

import "testing"

const derivedNameCollisionSpec = `openapi: 3.1.0
info: { title: coll, version: "1" }
webhooks:
  pet-created:
    post:
      requestBody:
        content:
          application/json: { schema: { $ref: "#/components/schemas/Pet" } }
  pet_created:
    post:
      requestBody:
        content:
          application/json: { schema: { $ref: "#/components/schemas/Pet" } }
paths:
  /a-b:
    get:
      responses: { "204": { description: ok } }
  /a_b:
    get:
      responses: { "204": { description: ok } }
  /dup:
    get:
      operationId: sameName
      responses: { "204": { description: ok } }
  /dup2:
    get:
      operationId: same-name
      responses: { "204": { description: ok } }
  /hdr:
    get:
      operationId: getHdr
      responses:
        "200":
          description: ok
          headers:
            x-trace: { schema: { type: string } }
            X_Trace: { schema: { type: string } }
          content:
            application/json: { schema: { $ref: "#/components/schemas/Pet" } }
components:
  schemas:
    Pet:
      type: object
      properties:
        name: { type: string }
`

// TestE2E_DerivedNameCollisions covers the identifiers built from spec names
// rather than from schemas: methods, response header fields, and webhook parse
// functions. Each collided into a redeclaration that broke the whole package.
func TestE2E_DerivedNameCollisions(t *testing.T) {
	files, _ := generateFromSpec(t, derivedNameCollisionSpec, "collapi")

	runGeneratedWireTest(t, files, "derivedcollision", `package collapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Both paths and both operation ids keep a method of their own, and each still
// requests the path it came from.
func TestCollidingOperationsKeepTheirPaths(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.GetAB(t.Context()); err != nil {
		t.Fatalf("GetAB: %v", err)
	}
	if err := c.GetAB2(t.Context()); err != nil {
		t.Fatalf("GetAB2: %v", err)
	}
	if err := c.SameName(t.Context()); err != nil {
		t.Fatalf("SameName: %v", err)
	}
	if err := c.SameName2(t.Context()); err != nil {
		t.Fatalf("SameName2: %v", err)
	}

	want := []string{"/a-b", "/a_b", "/dup", "/dup2"}
	for i, w := range want {
		if asked[i] != w {
			t.Errorf("call %d requested %q, want %q", i, asked[i], w)
		}
	}
}

// Header names are distinct spec keys even when they normalize to one field.
func TestCollidingHeadersAreReadable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-trace", "first")
		w.Header().Set("X_Trace", "second")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"name": "Rex"})
	}))
	defer srv.Close()

	ctx, meta := WithResponseCapture(t.Context())
	if _, err := NewClient(srv.URL).GetHdr(ctx); err != nil {
		t.Fatalf("GetHdr: %v", err)
	}
	h := meta.GetHdrHeaders()
	if h.XTrace != "first" || h.XTrace2 != "second" {
		t.Errorf("headers = %+v, want each spec key in its own field", h)
	}
}

// Each webhook keeps a parse function of its own, and the dispatcher still
// routes both names.
func TestCollidingWebhooksBothParse(t *testing.T) {
	if _, err := ParsePetCreatedWebhook([]byte(`+"`"+`{"name":"a"}`+"`"+`)); err != nil {
		t.Fatalf("ParsePetCreatedWebhook: %v", err)
	}
	if _, err := ParsePetCreated2Webhook([]byte(`+"`"+`{"name":"b"}`+"`"+`)); err != nil {
		t.Fatalf("ParsePetCreated2Webhook: %v", err)
	}
	for _, name := range []string{"pet-created", "pet_created"} {
		if _, err := ParseWebhook(name, []byte(`+"`"+`{"name":"c"}`+"`"+`)); err != nil {
			t.Errorf("ParseWebhook(%q): %v", name, err)
		}
	}
}
`)
}
