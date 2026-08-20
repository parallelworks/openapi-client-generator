package generator

import "testing"

const unionParamSpec = `openapi: 3.1.0
info: { title: search, version: "1" }
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - name: either
          in: query
          schema:
            anyOf: [{ type: string }, { type: integer }]
        - name: ids
          in: query
          schema:
            anyOf:
              - { type: array, items: { type: string } }
              - { type: string }
        - name: X-Either
          in: header
          schema:
            anyOf: [{ type: string }, { type: integer }]
        - name: plain
          in: query
          schema: { type: string }
        - name: at
          in: query
          style: deepObject
          explode: true
          schema:
            type: object
            properties:
              lat: { type: number }
              lon: { type: number }
            required: [lat, lon]
      responses:
        "204": { description: ok }
  /items/{id}:
    get:
      operationId: getItem
      parameters:
        - name: id
          in: path
          required: true
          schema:
            anyOf: [{ type: string }, { type: integer }]
      responses:
        "204": { description: ok }
`

// TestE2E_UnionParamsSendTheValue covers a parameter whose schema is a union with
// a real choice. Typing it is only safe if the encoders write the value the union
// carries rather than the wrapper carrying it.
func TestE2E_UnionParamsSendTheValue(t *testing.T) {
	files, _ := generateFromSpec(t, unionParamSpec, "searchapi")

	runGeneratedWireTest(t, files, "unionparam", `package searchapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func capture(t *testing.T) (*Client, *http.Request) {
	t.Helper()
	var got http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = *r
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL), &got
}

func TestScalarVariantGoesOutAsItself(t *testing.T) {
	client, got := capture(t)

	either := ListItemsEither{Value: 42}
	if err := client.ListItems(t.Context(), ListItemsParams{Either: &either}); err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if q := got.URL.Query().Get("either"); q != "42" {
		t.Errorf("either = %q, want 42 rather than the wrapper's shape", q)
	}
}

func TestStringVariantGoesOutAsItself(t *testing.T) {
	client, got := capture(t)

	either := ListItemsEither{Value: "abc"}
	if err := client.ListItems(t.Context(), ListItemsParams{Either: &either}); err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if q := got.URL.Query().Get("either"); q != "abc" {
		t.Errorf("either = %q, want abc", q)
	}
}

// A list variant still explodes the way the style says, since unwrapping happens
// before the style is applied rather than instead of it.
func TestListVariantKeepsItsStyle(t *testing.T) {
	client, got := capture(t)

	ids := ListItemsIds{Value: []string{"a", "b"}}
	if err := client.ListItems(t.Context(), ListItemsParams{Ids: &ids}); err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if values := got.URL.Query()["ids"]; len(values) != 2 || values[0] != "a" || values[1] != "b" {
		t.Errorf("ids = %v, want the exploded list", values)
	}
}

func TestHeaderAndPathUnwrapToo(t *testing.T) {
	client, got := capture(t)

	// One shape is one type, so the header parameter shares the union the query
	// parameter named rather than declaring a second identical wrapper.
	either := ListItemsEither{Value: 7}
	if err := client.ListItems(t.Context(), ListItemsParams{XEither: &either}); err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if h := got.Header.Get("X-Either"); h != "7" {
		t.Errorf("X-Either = %q, want 7", h)
	}

	client, got = capture(t)
	if err := client.GetItem(t.Context(), ListItemsEither{Value: "abc"}); err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if got.URL.Path != "/items/abc" {
		t.Errorf("path = %q, want the value in place of the wrapper", got.URL.Path)
	}
}

// An empty union sends nothing rather than a zero of some variant's type.
func TestEmptyUnionIsOmitted(t *testing.T) {
	client, got := capture(t)

	var either ListItemsEither
	if err := client.ListItems(t.Context(), ListItemsParams{Either: &either}); err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if _, ok := got.URL.Query()["either"]; ok {
		t.Errorf("query = %q, want no parameter for a union carrying nothing", got.URL.RawQuery)
	}
}

// An object written inline in a parameter is hintless for the same reason a
// union is, so it is typed by the same change and still encodes under its style.
func TestInlineObjectParamIsTypedAndStyled(t *testing.T) {
	client, got := capture(t)

	at := ListItemsAt{Lat: 51.5, Lon: -0.12}
	if err := client.ListItems(t.Context(), ListItemsParams{At: &at}); err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	q := got.URL.Query()
	if q.Get("at[lat]") != "51.5" || q.Get("at[lon]") != "-0.12" {
		t.Errorf("query = %q, want the object exploded under deepObject", got.URL.RawQuery)
	}
}

func TestPlainParamIsUnaffected(t *testing.T) {
	client, got := capture(t)

	value := "hello"
	if err := client.ListItems(t.Context(), ListItemsParams{Plain: &value}); err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if q := got.URL.Query().Get("plain"); q != "hello" {
		t.Errorf("plain = %q", q)
	}
}
`)
}
