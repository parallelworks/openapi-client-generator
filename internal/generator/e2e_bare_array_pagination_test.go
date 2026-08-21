package generator

import "testing"

const bareArrayPaginationSpec = `openapi: 3.1.0
info: { title: alerts, version: "1" }
paths:
  /alerts:
    get:
      operationId: listAlerts
      parameters:
        - { name: skip, in: query, schema: { type: integer } }
        - { name: limit, in: query, schema: { type: integer } }
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { type: array, items: { $ref: "#/components/schemas/Alert" } }
components:
  schemas:
    Alert:
      type: object
      properties:
        id: { type: string }
`

// TestE2E_BareArrayPagination covers an endpoint that returns the collection
// directly. Detection looked for the items inside a response object, so an
// operation whose parameters plainly page got no iterator.
func TestE2E_BareArrayPagination(t *testing.T) {
	files, _ := generateFromSpec(t, bareArrayPaginationSpec, "alertsapi")

	runGeneratedWireTest(t, files, "barearray", `package alertsapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIteratesTheResponseItself(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		skip := r.URL.Query().Get("skip")
		asked = append(asked, skip)
		bySkip := map[string][]map[string]string{
			"":  {{"id": "a"}, {"id": "b"}},
			"2": {{"id": "c"}},
			"3": {},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bySkip[skip])
	}))
	defer srv.Close()

	all, err := NewClient(srv.URL).ListAlertsIter(t.Context()).All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("items = %d, want 3", len(all))
	}
	if all[0].ID == nil || *all[0].ID != "a" || *all[2].ID != "c" {
		t.Errorf("items = %+v, want a, b, c", all)
	}
	if len(asked) != 3 || asked[1] != "2" || asked[2] != "3" {
		t.Errorf("skips requested = %v, want the first page, then 2, then 3", asked)
	}
}

// A page that comes back as JSON null is empty, not a crash.
func TestNullPageEndsTheWalk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("null"))
	}))
	defer srv.Close()

	all, err := NewClient(srv.URL).ListAlertsIter(t.Context()).All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("items = %d, want none", len(all))
	}
}
`)
}
