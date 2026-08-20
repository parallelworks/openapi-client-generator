package generator

import "testing"

const contentParamSpec = `openapi: 3.1.0
info: { title: search, version: "1" }
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - name: filter
          in: query
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Filter" }
        - name: X-Context
          in: header
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Filter" }
        - name: plain
          in: query
          schema: { type: string }
        - name: inline
          in: query
          content:
            application/json:
              schema:
                type: object
                properties:
                  page: { type: integer }
                required: [page]
      responses:
        "204": { description: ok }
components:
  schemas:
    Filter:
      type: object
      properties:
        field: { type: string }
        values:
          type: array
          items: { type: string }
      required: [field]
`

// TestE2E_ContentParamIsSerializedAsItsMediaType covers a parameter that names a
// media type instead of a style. Its value is a document, and sending it
// style-encoded is not what the server parses.
func TestE2E_ContentParamIsSerializedAsItsMediaType(t *testing.T) {
	files, _ := generateFromSpec(t, contentParamSpec, "searchapi")

	runGeneratedWireTest(t, files, "contentparam", `package searchapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONDocumentReachesTheServer(t *testing.T) {
	var rawQuery, header string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.Query().Get("filter")
		header = r.Header.Get("X-Context")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	filter := Filter{Field: "name", Values: []string{"a", "b"}}
	err := NewClient(srv.URL).ListItems(t.Context(), ListItemsParams{
		Filter:   &filter,
		XContext: &filter,
	})
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}

	// The server decodes the parameter as the document it declared.
	var got Filter
	if err := json.Unmarshal([]byte(rawQuery), &got); err != nil {
		t.Fatalf("query parameter was not the declared media type: %q: %v", rawQuery, err)
	}
	if got.Field != "name" || len(got.Values) != 2 {
		t.Errorf("decoded filter = %+v, want the value that was sent", got)
	}

	var fromHeader Filter
	if err := json.Unmarshal([]byte(header), &fromHeader); err != nil {
		t.Fatalf("header parameter was not the declared media type: %q: %v", header, err)
	}
	if fromHeader.Field != "name" {
		t.Errorf("decoded header filter = %+v", fromHeader)
	}
}

// The schema behind a content parameter is typed like any other, including one
// written inline.
func TestInlineContentParamIsTyped(t *testing.T) {
	var raw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw = r.URL.Query().Get("inline")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	inline := ListItemsInline{Page: 3}
	if err := NewClient(srv.URL).ListItems(t.Context(), ListItemsParams{Inline: &inline}); err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if raw != `+"`"+`{"page":3}`+"`"+` {
		t.Errorf("inline = %q, want the serialized document", raw)
	}
}

// A style-encoded parameter alongside it keeps its own encoding.
func TestStyleParamIsUnaffected(t *testing.T) {
	var plain string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plain = r.URL.Query().Get("plain")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	value := "hello"
	if err := NewClient(srv.URL).ListItems(t.Context(), ListItemsParams{Plain: &value}); err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if plain != "hello" {
		t.Errorf("plain = %q, want hello unquoted", plain)
	}
}

// An omitted optional parameter sends nothing, not "null".
func TestAbsentContentParamIsOmitted(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := NewClient(srv.URL).ListItems(t.Context()); err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if query != "" {
		t.Errorf("query = %q, want no parameters at all", query)
	}
}
`)
}
