package generator

import (
	"strings"
	"testing"
)

const templatedServerAPISpec = `openapi: 3.1.0
info: { title: regional, version: "1" }
servers:
  - url: https://{region}.api.example.com/{basePath}
    variables:
      region: { default: us-east-1, enum: [us-east-1, eu-west-1] }
      basePath: { default: v2 }
paths:
  /things:
    get:
      operationId: listThings
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { type: array, items: { type: string } }
`

// TestE2E_ServerURLFromSpec covers the URL the spec states: a caller should not
// have to paste it, and a templated one is unusable until its variables are
// substituted.
func TestE2E_ServerURLFromSpec(t *testing.T) {
	files, _ := generateFromSpec(t, templatedServerAPISpec, "regional")

	runGeneratedWireTest(t, files, "serverurl", `package regional

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDefaultBaseURLTakesTheVariableDefaults(t *testing.T) {
	if DefaultBaseURL != "https://us-east-1.api.example.com/v2" {
		t.Errorf("DefaultBaseURL = %q, want the declared URL with its defaults", DefaultBaseURL)
	}
}

func TestServerURLSubstitutes(t *testing.T) {
	if got := ServerURL("eu-west-1", "v3"); got != "https://eu-west-1.api.example.com/v3" {
		t.Errorf("ServerURL = %q", got)
	}
	if got := ServerURL("", ""); got != DefaultBaseURL {
		t.Errorf("ServerURL with empty arguments = %q, want %q", got, DefaultBaseURL)
	}
	if got := ServerURL("eu-west-1", ""); got != "https://eu-west-1.api.example.com/v2" {
		t.Errorf("ServerURL with one argument = %q", got)
	}
}

// The built URL has to work as a base URL, not just read like one.
func TestBuiltURLDrivesTheClient(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode([]string{"a"})
	}))
	defer srv.Close()

	base := strings.Replace(ServerURL("eu-west-1", "v3"), "https://eu-west-1.api.example.com", srv.URL, 1)
	things, err := NewClient(base).ListThings(t.Context())
	if err != nil {
		t.Fatalf("ListThings: %v", err)
	}
	if gotPath != "/v3/things" {
		t.Errorf("requested %q, want the server path joined to the operation path", gotPath)
	}
	if things == nil || len((*things)) != 1 {
		t.Errorf("things = %v, want one item", things)
	}
}
`)
}

const relativeServerSpec = `openapi: 3.1.0
info: { title: rel, version: "1" }
servers:
  - url: /api/v3
paths: {}
`

// A relative server URL is resolved against wherever the spec is served, so
// there is no constant that stands in for it.
func TestE2E_RelativeServerHasNoDefaultBaseURL(t *testing.T) {
	files, _ := generateFromSpec(t, relativeServerSpec, "relapi")

	for _, f := range files {
		if f.Name == "client.go" && strings.Contains(string(f.Content), "DefaultBaseURL") {
			t.Error("a relative server URL produced a DefaultBaseURL constant")
		}
	}
	buildGenerated(t, files, "relserver")
}
