package generator

import "testing"

const responseHeaderSpec = `openapi: 3.1.0
info: { title: pets, version: "1" }
paths:
  /pets:
    post:
      operationId: createPet
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: "#/components/schemas/Pet" }
      responses:
        "201":
          description: created
          headers:
            Location:
              description: Where the new pet lives.
              schema: { type: string }
            X-Rate-Limit-Remaining:
              schema: { type: integer }
            X-Deprecated:
              schema: { type: boolean }
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Pet" }
        "429":
          description: slow down
          headers:
            X-Rate-Limit-Remaining:
              schema: { type: integer }
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Pet" }
  /pets/{id}:
    get:
      operationId: getPet
      parameters:
        - { name: id, in: path, required: true, schema: { type: string } }
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Pet" }
components:
  schemas:
    Pet:
      type: object
      properties:
        name: { type: string }
`

// TestE2E_ResponseHeaderCapture covers what a response says outside its body:
// the status and headers were read inside do and discarded, so a caller could
// not reach Location on a 201 or a rate limit on a 429.
func TestE2E_ResponseHeaderCapture(t *testing.T) {
	files, _ := generateFromSpec(t, responseHeaderSpec, "petsapi")

	runGeneratedWireTest(t, files, "respheaders", `package petsapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newServer(t *testing.T, status int, headers map[string]string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{"name": "Rex"})
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL)
}

func TestCaptureReadsDeclaredHeaders(t *testing.T) {
	client := newServer(t, http.StatusCreated, map[string]string{
		"Location":               "/pets/1",
		"X-Rate-Limit-Remaining": "42",
		"X-Deprecated":           "true",
	})

	ctx, meta := WithResponseCapture(t.Context())
	if _, err := client.CreatePet(ctx, Pet{}); err != nil {
		t.Fatalf("CreatePet: %v", err)
	}

	if meta.StatusCode != http.StatusCreated {
		t.Errorf("StatusCode = %d, want 201", meta.StatusCode)
	}
	h := meta.CreatePetHeaders()
	if h.Location != "/pets/1" {
		t.Errorf("Location = %q, want /pets/1", h.Location)
	}
	if h.XRateLimitRemaining == nil || *h.XRateLimitRemaining != 42 {
		t.Errorf("XRateLimitRemaining = %v, want 42", h.XRateLimitRemaining)
	}
	if h.XDeprecated == nil || !*h.XDeprecated {
		t.Errorf("XDeprecated = %v, want true", h.XDeprecated)
	}
}

// A header the response omits is nil rather than a zero that reads as a real
// value, and an undeclared header is still reachable raw.
func TestAbsentHeaderIsNilAndUndeclaredIsRaw(t *testing.T) {
	client := newServer(t, http.StatusCreated, map[string]string{"X-Trace-Id": "abc123"})

	ctx, meta := WithResponseCapture(t.Context())
	if _, err := client.CreatePet(ctx, Pet{}); err != nil {
		t.Fatalf("CreatePet: %v", err)
	}

	h := meta.CreatePetHeaders()
	if h.XRateLimitRemaining != nil {
		t.Errorf("XRateLimitRemaining = %v, want nil when the response omits it", *h.XRateLimitRemaining)
	}
	if h.Location != "" {
		t.Errorf("Location = %q, want empty", h.Location)
	}
	if meta.Header.Get("X-Trace-Id") != "abc123" {
		t.Error("a header the spec does not declare should still be readable")
	}
}

// The rate limit that matters most arrives with the error, so the capture has to
// survive a failed call.
func TestErrorResponseIsCaptured(t *testing.T) {
	client := newServer(t, http.StatusTooManyRequests, map[string]string{"X-Rate-Limit-Remaining": "0"})

	ctx, meta := WithResponseCapture(t.Context())
	_, err := client.CreatePet(ctx, Pet{})
	if err == nil {
		t.Fatal("expected an error for 429")
	}

	if meta.StatusCode != http.StatusTooManyRequests {
		t.Errorf("StatusCode = %d, want 429", meta.StatusCode)
	}
	if h := meta.CreatePetHeaders(); h.XRateLimitRemaining == nil || *h.XRateLimitRemaining != 0 {
		t.Errorf("XRateLimitRemaining = %v, want 0", h.XRateLimitRemaining)
	}
}

// A call made without a capture must behave exactly as before.
func TestUncapturedCallIsUnaffected(t *testing.T) {
	client := newServer(t, http.StatusOK, nil)

	pet, err := client.GetPet(t.Context(), "1")
	if err != nil {
		t.Fatalf("GetPet: %v", err)
	}
	if pet == nil || pet.Name == nil || *pet.Name != "Rex" {
		t.Errorf("pet = %+v, want the decoded body", pet)
	}
}
`)
}
