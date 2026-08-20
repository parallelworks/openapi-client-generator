package analyzer

import (
	"path/filepath"
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
	"github.com/parallelworks/openapi-client-generator/internal/parser"
)

func TestPagination_ListPetsDetectedAsCursor(t *testing.T) {
	specPath := filepath.Join(projectRoot(), "testdata", "petstore.yaml")
	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	a := New(result.Model)
	pkg, err := a.Analyze("petstore")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	opMap := make(map[string]*ir.OperationDef)
	for _, op := range pkg.Operations {
		opMap[op.Name] = op
	}

	// listPets should be detected as cursor-based pagination.
	listPets := opMap["ListPets"]
	if listPets == nil {
		t.Fatal("ListPets operation not found")
	}
	if listPets.Pagination == nil {
		t.Fatal("ListPets.Pagination should not be nil")
	}
	if listPets.Pagination.Style != ir.PaginationStyleCursor {
		t.Errorf("ListPets.Pagination.Style = %v, want PaginationStyleCursor", listPets.Pagination.Style)
	}
	if listPets.Pagination.CursorParam != "cursor" {
		t.Errorf("ListPets.Pagination.CursorParam = %q, want %q", listPets.Pagination.CursorParam, "cursor")
	}
	if listPets.Pagination.CursorField != "nextCursor" {
		t.Errorf("ListPets.Pagination.CursorField = %q, want %q", listPets.Pagination.CursorField, "nextCursor")
	}
	if listPets.Pagination.ItemsField != "items" {
		t.Errorf("ListPets.Pagination.ItemsField = %q, want %q", listPets.Pagination.ItemsField, "items")
	}
}

func TestPagination_NonPaginatedOps(t *testing.T) {
	specPath := filepath.Join(projectRoot(), "testdata", "petstore.yaml")
	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	a := New(result.Model)
	pkg, err := a.Analyze("petstore")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	opMap := make(map[string]*ir.OperationDef)
	for _, op := range pkg.Operations {
		opMap[op.Name] = op
	}

	// These operations should NOT be paginated.
	for _, name := range []string{"CreatePet", "GetPetByID", "DeletePet"} {
		op := opMap[name]
		if op == nil {
			t.Fatalf("%s operation not found", name)
		}
		if op.Pagination != nil {
			t.Errorf("%s should not be paginated, got %+v", name, op.Pagination)
		}
	}
}

func TestPagination_OffsetDetection(t *testing.T) {
	// Create a minimal IR package with offset-style params to test detection.
	pkg := &ir.Package{
		Name: "test",
		Types: []*ir.TypeDef{
			{
				Name: "ItemList",
				Kind: ir.TypeKindStruct,
				Fields: []*ir.Field{
					{Name: "Items", JSONName: "items", Type: "[]Item"},
					{Name: "Total", JSONName: "total", Type: "int64"},
				},
			},
		},
		Operations: []*ir.OperationDef{
			{
				Name:       "ListItems",
				HTTPMethod: "GET",
				Path:       "/items",
				QueryParams: []*ir.ParamDef{
					{Name: "offset", OrigName: "offset", Location: "query", Type: "int64"},
					{Name: "limit", OrigName: "limit", Location: "query", Type: "int64"},
				},
				SuccessResponse: &ir.ResponseDef{
					StatusCode: "200",
					TypeName:   "ItemList",
				},
			},
		},
	}

	a := &Analyzer{typesBySchema: make(map[string]*ir.TypeDef)}
	a.detectPagination(pkg)

	op := pkg.Operations[0]
	if op.Pagination == nil {
		t.Fatal("ListItems.Pagination should not be nil")
	}
	if op.Pagination.Style != ir.PaginationStyleOffset {
		t.Errorf("ListItems.Pagination.Style = %v, want PaginationStyleOffset", op.Pagination.Style)
	}
	if op.Pagination.OffsetParam != "offset" {
		t.Errorf("ListItems.Pagination.OffsetParam = %q, want %q", op.Pagination.OffsetParam, "offset")
	}
	if op.Pagination.LimitParam != "limit" {
		t.Errorf("ListItems.Pagination.LimitParam = %q, want %q", op.Pagination.LimitParam, "limit")
	}
	if op.Pagination.ItemsField != "items" {
		t.Errorf("ListItems.Pagination.ItemsField = %q, want %q", op.Pagination.ItemsField, "items")
	}
}

func TestPagination_PageBasedDetection(t *testing.T) {
	pkg := &ir.Package{
		Name: "test",
		Types: []*ir.TypeDef{
			{
				Name: "ResultList",
				Kind: ir.TypeKindStruct,
				Fields: []*ir.Field{
					{Name: "Results", JSONName: "results", Type: "[]Result"},
				},
			},
		},
		Operations: []*ir.OperationDef{
			{
				Name:       "ListResults",
				HTTPMethod: "GET",
				Path:       "/results",
				QueryParams: []*ir.ParamDef{
					{Name: "page", OrigName: "page", Location: "query", Type: "int64"},
					{Name: "perPage", OrigName: "per_page", Location: "query", Type: "int64"},
				},
				SuccessResponse: &ir.ResponseDef{
					StatusCode: "200",
					TypeName:   "ResultList",
				},
			},
		},
	}

	a := &Analyzer{typesBySchema: make(map[string]*ir.TypeDef)}
	a.detectPagination(pkg)

	op := pkg.Operations[0]
	if op.Pagination == nil {
		t.Fatal("ListResults.Pagination should not be nil")
	}
	// page and per_page count pages, not items, so they advance by one.
	if op.Pagination.Style != ir.PaginationStylePage {
		t.Errorf("ListResults.Pagination.Style = %v, want PaginationStylePage", op.Pagination.Style)
	}
	if op.Pagination.OffsetParam != "page" {
		t.Errorf("ListResults.Pagination.OffsetParam = %q, want %q", op.Pagination.OffsetParam, "page")
	}
	if op.Pagination.LimitParam != "per_page" {
		t.Errorf("ListResults.Pagination.LimitParam = %q, want %q", op.Pagination.LimitParam, "per_page")
	}
	if op.Pagination.ItemsField != "results" {
		t.Errorf("ListResults.Pagination.ItemsField = %q, want %q", op.Pagination.ItemsField, "results")
	}
}

func TestContainsCI(t *testing.T) {
	tests := []struct {
		list   []string
		target string
		want   bool
	}{
		{[]string{"cursor", "after"}, "cursor", true},
		{[]string{"cursor", "after"}, "CURSOR", true},
		{[]string{"cursor", "after"}, "Cursor", true},
		{[]string{"cursor", "after"}, "before", false},
		{[]string{}, "cursor", false},
	}

	for _, tt := range tests {
		got := containsCI(tt.list, tt.target)
		if got != tt.want {
			t.Errorf("containsCI(%v, %q) = %v, want %v", tt.list, tt.target, got, tt.want)
		}
	}
}

// itemsSpec builds a cursor-paginated operation whose page type declares the
// given properties, so each case differs only in what the response holds.
func itemsSpec(properties string) string {
	return `openapi: 3.1.0
info: { title: t, version: "1" }
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
        nextCursor: { type: string }
` + properties + `
    Event:
      type: object
      properties: { id: { type: string } }
`
}

func paginationOf(t *testing.T, spec string) *ir.PaginationDef {
	t.Helper()
	pkg, _ := analyzeSpec(t, spec)
	for _, op := range pkg.Operations {
		if op.Name == "ListEvents" {
			return op.Pagination
		}
	}
	t.Fatal("ListEvents operation not found")
	return nil
}

func TestPaginationItems_ConventionalNameWins(t *testing.T) {
	pd := paginationOf(t, itemsSpec(`        data: { type: array, items: { $ref: "#/components/schemas/Event" } }
        warnings: { type: array, items: { type: string } }`))

	if pd == nil || pd.ItemsField != "data" {
		t.Errorf("items field = %+v, want data", pd)
	}
}

func TestPaginationItems_LoneArrayIsThePage(t *testing.T) {
	pd := paginationOf(t, itemsSpec(`        events: { type: array, items: { $ref: "#/components/schemas/Event" } }`))

	if pd == nil || pd.ItemsField != "events" || pd.ItemsType != "Event" {
		t.Errorf("items field = %+v, want events of Event", pd)
	}
}

// A page holds records, so an array of a declared type beside an array of
// scalars still identifies itself.
func TestPaginationItems_RecordArrayBeatsScalarArrays(t *testing.T) {
	pd := paginationOf(t, itemsSpec(`        warnings: { type: array, items: { type: string } }
        events: { type: array, items: { $ref: "#/components/schemas/Event" } }`))

	if pd == nil || pd.ItemsField != "events" {
		t.Errorf("items field = %+v, want events rather than the first array declared", pd)
	}
}

// Two arrays of records identify nothing. Paging over whichever the spec
// declares first returns the wrong data, so the operation gets no iterator.
func TestPaginationItems_AmbiguousArraysGetNoIterator(t *testing.T) {
	pd := paginationOf(t, itemsSpec(`        warnings: { type: array, items: { $ref: "#/components/schemas/Event" } }
        events: { type: array, items: { $ref: "#/components/schemas/Event" } }`))

	if pd != nil {
		t.Errorf("pagination = %+v, want none: neither array identifies the page", pd)
	}
}

func TestPaginationItems_NoArrayGetsNoIterator(t *testing.T) {
	pd := paginationOf(t, itemsSpec(`        total: { type: integer }`))

	if pd != nil {
		t.Errorf("pagination = %+v, want none: the response holds no page", pd)
	}
}
