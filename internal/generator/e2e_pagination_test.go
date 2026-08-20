package generator

import "testing"

const cursorPageSpec = `openapi: 3.1.0
info: { title: events, version: "1" }
paths:
  /events:
    get:
      operationId: listEvents
      parameters:
        - { name: cursor, in: query, schema: { type: string } }
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/EventPage" }
components:
  schemas:
    EventPage:
      type: object
      properties:
        cursor: { type: string }
        events: { type: array, items: { $ref: "#/components/schemas/Event" } }
    Event:
      type: object
      properties:
        id: { type: string }
`

// TestE2E_IteratorStopsOnANonAdvancingCursor covers a server that echoes the
// cursor it was given, which a response field named `cursor` commonly is. The
// iterator stopped only on an empty cursor, so All() ran identical requests
// forever.
func TestE2E_IteratorStopsOnANonAdvancingCursor(t *testing.T) {
	files, _ := generateFromSpec(t, cursorPageSpec, "eventsapi")

	runGeneratedWireTest(t, files, "cursorpage", `package eventsapi

import "testing"

func TestEchoedCursorTerminates(t *testing.T) {
	calls := 0
	it := &PageIterator[Event]{
		fetch: func(cursor string) ([]Event, string, error) {
			calls++
			if calls > 100 {
				t.Fatal("iterator never stopped on a cursor that does not advance")
			}
			return []Event{{}}, "same-cursor", nil
		},
	}

	items, err := it.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if calls != 2 {
		t.Errorf("fetched %d pages, want 2: one page, then the repeat that ends it", calls)
	}
	if len(items) != 2 {
		t.Errorf("items = %d, want the 2 pages it did fetch", len(items))
	}
}

func TestAdvancingCursorPagesToTheEnd(t *testing.T) {
	pages := map[string]string{"": "b", "b": "c", "c": ""}
	var seen []string

	it := &PageIterator[Event]{
		fetch: func(cursor string) ([]Event, string, error) {
			seen = append(seen, cursor)
			return []Event{{}}, pages[cursor], nil
		},
	}

	items, err := it.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("items = %d, want 3", len(items))
	}
	if len(seen) != 3 || seen[1] != "b" || seen[2] != "c" {
		t.Errorf("cursors sent = %v, want the empty first page then b and c", seen)
	}
}
`)
}
