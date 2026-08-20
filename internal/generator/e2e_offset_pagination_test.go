package generator

import "testing"

const offsetPaginationSpec = `openapi: 3.1.0
info: { title: pag, version: "1" }
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - { name: offset, in: query, schema: { type: integer } }
        - { name: limit, in: query, schema: { type: integer } }
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { $ref: "#/components/schemas/Page" } }
  /results:
    get:
      operationId: listResults
      parameters:
        - { name: page, in: query, schema: { type: integer } }
        - { name: per_page, in: query, schema: { type: integer } }
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { $ref: "#/components/schemas/Page" } }
components:
  schemas:
    Page:
      type: object
      properties:
        items: { type: array, items: { $ref: "#/components/schemas/Item" } }
        total: { type: integer }
    Item:
      type: object
      properties:
        id: { type: string }
`

// TestE2E_OffsetAndPageIterators covers the two counting styles. Both were
// detected and neither generated an iterator, so the package exported
// PageIterator[T] with no method returning one.
func TestE2E_OffsetAndPageIterators(t *testing.T) {
	files, _ := generateFromSpec(t, offsetPaginationSpec, "pagapi")

	runGeneratedWireTest(t, files, "offsetpagination", `package pagapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// pages serves items keyed by the value of param, and records what was asked for.
func pages(t *testing.T, param string, byValue map[string][]string) (*Client, *[]string) {
	t.Helper()
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := r.URL.Query().Get(param)
		asked = append(asked, value)
		items := []map[string]string{}
		for _, id := range byValue[value] {
			items = append(items, map[string]string{"id": id})
		}
		json.NewEncoder(w).Encode(map[string]any{"items": items})
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL), &asked
}

// An offset advances by the number of items received.
func TestOffsetIteratorCountsItems(t *testing.T) {
	client, asked := pages(t, "offset", map[string][]string{
		"":  {"a", "b"},
		"2": {"c"},
		"3": {},
	})

	all, err := client.ListItemsIter(t.Context()).All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 3 || all[0].ID == nil || *all[0].ID != "a" || *all[2].ID != "c" {
		t.Errorf("items = %+v, want a, b, c", all)
	}
	if len(*asked) != 3 || (*asked)[1] != "2" || (*asked)[2] != "3" {
		t.Errorf("offsets requested = %v, want the first page, then 2, then 3", *asked)
	}
}

// A page number advances by one, whatever a page holds.
func TestPageIteratorCountsPages(t *testing.T) {
	client, asked := pages(t, "page", map[string][]string{
		"":  {"a", "b"},
		"2": {"c", "d"},
		"3": {},
	})

	all, err := client.ListResultsIter(t.Context()).All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("items = %d, want 4", len(all))
	}
	if len(*asked) != 3 || (*asked)[1] != "2" || (*asked)[2] != "3" {
		t.Errorf("pages requested = %v, want the first page, then 2, then 3", *asked)
	}
}

// A caller who points the iterator at a starting offset gets a walk that
// continues from there, rather than one that resets to the beginning on the
// second page.
func TestCallerStartIsHonoredAndContinued(t *testing.T) {
	client, asked := pages(t, "offset", map[string][]string{
		"10": {"k", "l"},
		"12": {"m"},
		"13": {},
	})

	start := int64(10)
	all, err := client.ListItemsIter(t.Context(), ListItemsParams{Offset: &start}).All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("items = %d, want 3", len(all))
	}
	if len(*asked) != 3 || (*asked)[0] != "10" || (*asked)[1] != "12" || (*asked)[2] != "13" {
		t.Errorf("offsets requested = %v, want 10, then 12, then 13", *asked)
	}
}

// An empty first page ends the walk rather than repeating it.
func TestEmptyFirstPageStops(t *testing.T) {
	client, asked := pages(t, "offset", map[string][]string{"": {}})

	all, err := client.ListItemsIter(t.Context()).All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 0 || len(*asked) != 1 {
		t.Errorf("items = %d after %v, want none after one request", len(all), *asked)
	}
}
`)
}
