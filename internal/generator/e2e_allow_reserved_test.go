package generator

import "testing"

const allowReservedSpec = `openapi: 3.1.0
info: { title: paths, version: "1" }
paths:
  /lookup:
    get:
      operationId: lookup
      parameters:
        - name: ref
          in: query
          allowReserved: true
          schema: { type: string }
        - name: escaped
          in: query
          schema: { type: string }
      responses:
        "204": { description: ok }
`

// TestE2E_AllowReservedKeepsReservedCharacters covers allowReserved, which says
// the reserved set is passed through rather than percent-encoded. A server that
// asks for a path-like value receives it escaped otherwise.
func TestE2E_AllowReservedKeepsReservedCharacters(t *testing.T) {
	files, _ := generateFromSpec(t, allowReservedSpec, "pathsapi")

	runGeneratedWireTest(t, files, "allowreserved", `package pathsapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func lookupQuery(t *testing.T, params LookupParams) (raw string, decoded map[string]string) {
	t.Helper()
	decoded = map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw = r.URL.RawQuery
		for k, v := range r.URL.Query() {
			decoded[k] = v[0]
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := NewClient(srv.URL).Lookup(t.Context(), params); err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	return raw, decoded
}

func TestReservedCharactersSurviveTheWire(t *testing.T) {
	value := "org/repo:main"
	raw, decoded := lookupQuery(t, LookupParams{Ref: &value})

	if !strings.Contains(raw, "ref=org/repo:main") {
		t.Errorf("raw query = %q, want the reserved characters unescaped", raw)
	}
	// Unescaped or not, the value still arrives intact.
	if decoded["ref"] != value {
		t.Errorf("server read %q, want %q", decoded["ref"], value)
	}
}

func TestOtherParamsStayEscaped(t *testing.T) {
	value := "org/repo:main"
	raw, decoded := lookupQuery(t, LookupParams{Escaped: &value})

	if strings.Contains(raw, "org/repo") {
		t.Errorf("raw query = %q, want the value escaped", raw)
	}
	if decoded["escaped"] != value {
		t.Errorf("server read %q, want %q", decoded["escaped"], value)
	}
}

// The characters a query string is parsed with stay escaped even under
// allowReserved: passing them through would end the value early, so the server
// would read something other than what was sent.
func TestQuerySeparatorsStayEscaped(t *testing.T) {
	value := "a&b=c#d+e"
	raw, decoded := lookupQuery(t, LookupParams{Ref: &value})

	if decoded["ref"] != value {
		t.Errorf("server read %q, want %q (raw: %q)", decoded["ref"], value, raw)
	}
}
`)
}
