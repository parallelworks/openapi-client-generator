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

const camelPageSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - { name: page, in: query, schema: { type: integer } }
        - { name: perPage, in: query, schema: { type: integer } }
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { $ref: "#/components/schemas/Page" } }
  /others:
    get:
      operationId: listOthers
      parameters:
        - { name: page, in: query, schema: { type: integer } }
        - { name: pageSize, in: query, schema: { type: integer } }
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
    Item:
      type: object
      properties: { id: { type: string } }
`

// Both spellings of a page size are recognized. perPage was not, so a spec
// written in camelCase, which is what several generators emit, paginated
// nowhere: the Mealie spec in #15 declares page and perPage on 30 endpoints.
func TestPagination_CamelCasePageSize(t *testing.T) {
	pkg, _ := analyzeSpec(t, camelPageSpec)

	for _, op := range pkg.Operations {
		if op.Pagination == nil {
			t.Errorf("%s has no pagination, want the page style", op.Name)
			continue
		}
		if op.Pagination.Style != ir.PaginationStylePage {
			t.Errorf("%s style = %v, want PaginationStylePage", op.Name, op.Pagination.Style)
		}
		if op.Pagination.OffsetParam != "page" {
			t.Errorf("%s advances %q, want page", op.Name, op.Pagination.OffsetParam)
		}
	}

	byName := map[string]string{}
	for _, op := range pkg.Operations {
		if op.Pagination != nil {
			byName[op.Name] = op.Pagination.LimitParam
		}
	}
	if byName["ListItems"] != "perPage" {
		t.Errorf("ListItems limit param = %q, want perPage", byName["ListItems"])
	}
	if byName["ListOthers"] != "pageSize" {
		t.Errorf("ListOthers limit param = %q, want pageSize", byName["ListOthers"])
	}
}

const skipOffsetSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /runs:
    get:
      operationId: listRuns
      parameters:
        - { name: skip, in: query, schema: { type: integer } }
        - { name: limit, in: query, schema: { type: integer } }
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { $ref: "#/components/schemas/RunList" } }
  /both:
    get:
      operationId: listBoth
      parameters:
        - { name: offset, in: query, schema: { type: integer } }
        - { name: skip, in: query, schema: { type: integer } }
        - { name: limit, in: query, schema: { type: integer } }
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { $ref: "#/components/schemas/RunList" } }
  /alone:
    get:
      operationId: listAlone
      parameters:
        - { name: skip, in: query, schema: { type: integer } }
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { $ref: "#/components/schemas/RunList" } }
components:
  schemas:
    RunList:
      type: object
      properties:
        runs: { type: array, items: { $ref: "#/components/schemas/Run" } }
        total: { type: integer }
    Run:
      type: object
      properties: { id: { type: string } }
`

// skip is the other spelling of an offset, and the convention 15 endpoints of
// one real spec use. It advances by items received, exactly as offset does.
func TestPagination_SkipIsAnOffset(t *testing.T) {
	pkg, _ := analyzeSpec(t, skipOffsetSpec)

	byName := map[string]*ir.PaginationDef{}
	for _, op := range pkg.Operations {
		byName[op.Name] = op.Pagination
	}

	runs := byName["ListRuns"]
	if runs == nil {
		t.Fatal("ListRuns has no pagination")
	}
	if runs.Style != ir.PaginationStyleOffset {
		t.Errorf("ListRuns style = %v, want PaginationStyleOffset", runs.Style)
	}
	if runs.OffsetParam != "skip" || runs.LimitParam != "limit" {
		t.Errorf("ListRuns advances %q by %q, want skip and limit", runs.OffsetParam, runs.LimitParam)
	}
	if runs.ItemsField != "runs" {
		t.Errorf("ListRuns items = %q, want runs", runs.ItemsField)
	}

	// A spec offering both names is answered with offset, the one the standard
	// spells out.
	if both := byName["ListBoth"]; both == nil || both.OffsetParam != "offset" {
		t.Errorf("ListBoth advances %+v, want offset", both)
	}

	// An advancing parameter with nothing to size a page is not pagination.
	if alone := byName["ListAlone"]; alone != nil {
		t.Errorf("ListAlone pagination = %+v, want none", alone)
	}
}

const bareArrayPageSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
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
  /named:
    get:
      operationId: listNamed
      parameters:
        - { name: offset, in: query, schema: { type: integer } }
        - { name: limit, in: query, schema: { type: integer } }
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { $ref: "#/components/schemas/AlertList" } }
  /scalar:
    get:
      operationId: listScalar
      parameters:
        - { name: offset, in: query, schema: { type: integer } }
        - { name: limit, in: query, schema: { type: integer } }
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { type: string } }
components:
  schemas:
    AlertList:
      type: array
      items: { $ref: "#/components/schemas/Alert" }
    Alert:
      type: object
      properties: { id: { type: string } }
`

// A response that is the array is the page. There is no field to name and
// nothing to choose between, which is why this needs no rule about which array
// counts.
func TestPagination_BareArrayResponseIsThePage(t *testing.T) {
	pkg, _ := analyzeSpec(t, bareArrayPageSpec)

	byName := map[string]*ir.PaginationDef{}
	for _, op := range pkg.Operations {
		byName[op.Name] = op.Pagination
	}

	alerts := byName["ListAlerts"]
	if alerts == nil {
		t.Fatal("ListAlerts has no pagination")
	}
	if !alerts.ItemsAreResponse {
		t.Error("ListAlerts should page over the response itself")
	}
	if alerts.ItemsField != "" {
		t.Errorf("ListAlerts items field = %q, want none", alerts.ItemsField)
	}
	if alerts.ItemsType != "Alert" {
		t.Errorf("ListAlerts items type = %q, want Alert", alerts.ItemsType)
	}

	// A spec may name the array, which is the same page behind an alias.
	named := byName["ListNamed"]
	if named == nil || !named.ItemsAreResponse || named.ItemsType != "Alert" {
		t.Errorf("ListNamed pagination = %+v, want the aliased array's element", named)
	}

	// A response that is neither a page object nor an array is not a page.
	if scalar := byName["ListScalar"]; scalar != nil {
		t.Errorf("ListScalar pagination = %+v, want none", scalar)
	}
}

const nonNumericPagingSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /search:
    get:
      operationId: search
      parameters:
        - { name: page, in: query, schema: { type: string, maxLength: 5000 } }
        - { name: limit, in: query, schema: { type: integer } }
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { $ref: "#/components/schemas/Page" } }
  /counted:
    get:
      operationId: counted
      parameters:
        - { name: page, in: query, schema: { type: integer, format: int32 } }
        - { name: limit, in: query, schema: { type: integer } }
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { $ref: "#/components/schemas/Page" } }
  /tokened:
    get:
      operationId: tokened
      parameters:
        - { name: cursor, in: query, schema: { type: integer } }
        - { name: limit, in: query, schema: { type: integer } }
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
        nextCursor: { type: string }
    Item:
      type: object
      properties: { id: { type: string } }
`

// The generated iterator reads its position as a number and writes the next one
// back, so a page carrying a token rather than a count is not one it can walk.
// Stripe's search endpoints declare page as a string, and generating for them
// produced code that did not compile.
func TestPagination_ParameterTypesMustMatchTheWalk(t *testing.T) {
	pkg, _ := analyzeSpec(t, nonNumericPagingSpec)

	byName := map[string]*ir.PaginationDef{}
	for _, op := range pkg.Operations {
		byName[op.Name] = op.Pagination
	}

	if search := byName["Search"]; search != nil {
		t.Errorf("Search pagination = %+v, want none: its page is a string", search)
	}
	counted := byName["Counted"]
	if counted == nil || counted.Style != ir.PaginationStylePage {
		t.Errorf("Counted pagination = %+v, want the page style", counted)
	}
	// A cursor is handed back as the string it arrived as, so an integer one is
	// not a cursor this can drive either.
	if tokened := byName["Tokened"]; tokened != nil && tokened.Style == ir.PaginationStyleCursor {
		t.Errorf("Tokened pagination = %+v, want no cursor style: its cursor is an integer", tokened)
	}
}
