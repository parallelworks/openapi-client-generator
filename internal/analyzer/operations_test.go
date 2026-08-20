package analyzer

import (
	"path/filepath"
	"strings"
	"testing"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
	"github.com/parallelworks/openapi-client-generator/internal/parser"
)

func TestEffectiveStyleExplode(t *testing.T) {
	b := func(v bool) *bool { return &v }
	tests := []struct {
		name        string
		in          string
		style       string
		explode     *bool
		wantStyle   string
		wantExplode bool
	}{
		{"query default", "query", "", nil, "form", true},
		{"query explode=false", "query", "", b(false), "form", false},
		{"query spaceDelimited default explode", "query", "spaceDelimited", nil, "spaceDelimited", false},
		{"query pipeDelimited explode=true", "query", "pipeDelimited", b(true), "pipeDelimited", true},
		{"header default", "header", "", nil, "simple", false},
		{"header explode=true", "header", "", b(true), "simple", true},
		{"path default", "path", "", nil, "simple", false},
		{"cookie default", "cookie", "", nil, "form", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &v3high.Parameter{In: tt.in, Style: tt.style, Explode: tt.explode}
			style, explode := effectiveStyleExplode(p)
			if style != tt.wantStyle || explode != tt.wantExplode {
				t.Errorf("effectiveStyleExplode(in=%s style=%q explode=%v) = (%q, %v), want (%q, %v)",
					tt.in, tt.style, tt.explode, style, explode, tt.wantStyle, tt.wantExplode)
			}
		})
	}
}

func TestAnalyzeOperations_Petstore(t *testing.T) {
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

	// Should have 4 operations: listPets, createPet, getPetById, deletePet.
	if len(pkg.Operations) != 4 {
		t.Fatalf("expected 4 operations, got %d", len(pkg.Operations))
	}

	opMap := make(map[string]*ir.OperationDef)
	for _, op := range pkg.Operations {
		opMap[op.Name] = op
		t.Logf("Operation: %s %s %s", op.HTTPMethod, op.Path, op.Name)
	}

	// Verify ListPets.
	listPets, ok := opMap["ListPets"]
	if !ok {
		t.Fatal("expected ListPets operation")
	}
	if listPets.HTTPMethod != "GET" {
		t.Errorf("ListPets.HTTPMethod = %q, want GET", listPets.HTTPMethod)
	}
	if listPets.Path != "/pets" {
		t.Errorf("ListPets.Path = %q, want /pets", listPets.Path)
	}
	if listPets.Summary != "List all pets" {
		t.Errorf("ListPets.Summary = %q, want %q", listPets.Summary, "List all pets")
	}
	if listPets.Description != "Returns a paginated list of all pets in the store." {
		t.Errorf("ListPets.Description = %q, want %q", listPets.Description, "Returns a paginated list of all pets in the store.")
	}
	if len(listPets.QueryParams) != 2 {
		t.Errorf("ListPets: expected 2 query params, got %d", len(listPets.QueryParams))
	} else {
		if listPets.QueryParams[0].OrigName != "limit" {
			t.Errorf("ListPets.QueryParams[0].OrigName = %q, want limit", listPets.QueryParams[0].OrigName)
		}
		if listPets.QueryParams[0].Type != "int32" {
			t.Errorf("ListPets.QueryParams[0].Type = %q, want int32", listPets.QueryParams[0].Type)
		}
		if listPets.QueryParams[0].Required {
			t.Error("ListPets limit param should not be required")
		}
		if listPets.QueryParams[0].Description != "How many items to return at one time (max 100)" {
			t.Errorf("ListPets limit param Description = %q, want %q", listPets.QueryParams[0].Description, "How many items to return at one time (max 100)")
		}
		if listPets.QueryParams[1].OrigName != "cursor" {
			t.Errorf("ListPets.QueryParams[1].OrigName = %q, want cursor", listPets.QueryParams[1].OrigName)
		}
		if listPets.QueryParams[1].Type != "string" {
			t.Errorf("ListPets.QueryParams[1].Type = %q, want string", listPets.QueryParams[1].Type)
		}
	}
	if listPets.SuccessResponse == nil {
		t.Fatal("ListPets: expected SuccessResponse")
	}
	if listPets.SuccessResponse.TypeName != "PetList" {
		t.Errorf("ListPets.SuccessResponse.TypeName = %q, want PetList", listPets.SuccessResponse.TypeName)
	}
	if len(listPets.ErrorResponses) != 1 {
		t.Errorf("ListPets: expected 1 error response, got %d", len(listPets.ErrorResponses))
	}

	// Verify CreatePet.
	createPet, ok := opMap["CreatePet"]
	if !ok {
		t.Fatal("expected CreatePet operation")
	}
	if createPet.HTTPMethod != "POST" {
		t.Errorf("CreatePet.HTTPMethod = %q, want POST", createPet.HTTPMethod)
	}
	if createPet.RequestBody == nil {
		t.Fatal("CreatePet: expected RequestBody")
	}
	if createPet.RequestBody.TypeName != "CreatePetRequest" {
		t.Errorf("CreatePet.RequestBody.TypeName = %q, want CreatePetRequest", createPet.RequestBody.TypeName)
	}
	if !createPet.RequestBody.Required {
		t.Error("CreatePet.RequestBody should be required")
	}
	if createPet.RequestBody.ContentType != "application/json" {
		t.Errorf("CreatePet.RequestBody.ContentType = %q, want application/json", createPet.RequestBody.ContentType)
	}

	// Verify GetPetByID.
	getPet, ok := opMap["GetPetByID"]
	if !ok {
		t.Fatal("expected GetPetByID operation")
	}
	if getPet.HTTPMethod != "GET" {
		t.Errorf("GetPetByID.HTTPMethod = %q, want GET", getPet.HTTPMethod)
	}
	if len(getPet.PathParams) != 1 {
		t.Fatalf("GetPetByID: expected 1 path param, got %d", len(getPet.PathParams))
	}
	if getPet.PathParams[0].OrigName != "petId" {
		t.Errorf("GetPetByID.PathParams[0].OrigName = %q, want petId", getPet.PathParams[0].OrigName)
	}
	if getPet.PathParams[0].Type != "int64" {
		t.Errorf("GetPetByID.PathParams[0].Type = %q, want int64", getPet.PathParams[0].Type)
	}
	if !getPet.PathParams[0].Required {
		t.Error("GetPetByID petId param should be required")
	}
	if getPet.SuccessResponse == nil {
		t.Fatal("GetPetByID: expected SuccessResponse")
	}
	if getPet.SuccessResponse.TypeName != "Pet" {
		t.Errorf("GetPetByID.SuccessResponse.TypeName = %q, want Pet", getPet.SuccessResponse.TypeName)
	}
	// Should have 404 + default error responses.
	if len(getPet.ErrorResponses) != 2 {
		t.Errorf("GetPetByID: expected 2 error responses, got %d", len(getPet.ErrorResponses))
	}

	// Verify DeletePet.
	deletePet, ok := opMap["DeletePet"]
	if !ok {
		t.Fatal("expected DeletePet operation")
	}
	if deletePet.HTTPMethod != "DELETE" {
		t.Errorf("DeletePet.HTTPMethod = %q, want DELETE", deletePet.HTTPMethod)
	}
	if len(deletePet.PathParams) != 1 {
		t.Fatalf("DeletePet: expected 1 path param, got %d", len(deletePet.PathParams))
	}
	// deletePet has 204 with no body.
	if deletePet.SuccessResponse != nil && deletePet.SuccessResponse.TypeName != "" {
		t.Errorf("DeletePet.SuccessResponse.TypeName = %q, want empty (no body)", deletePet.SuccessResponse.TypeName)
	}
}

func TestAnalyzeOperations_TextPlain(t *testing.T) {
	specPath := filepath.Join(projectRoot(), "testdata", "text-plain.yaml")

	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	a := New(result.Model)
	pkg, err := a.Analyze("textplain")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	opMap := make(map[string]*ir.OperationDef)
	for _, op := range pkg.Operations {
		opMap[op.Name] = op
	}

	// Whoami: text/plain with schema type: string
	whoami, ok := opMap["Whoami"]
	if !ok {
		t.Fatal("expected Whoami operation")
	}
	if whoami.SuccessResponse == nil {
		t.Fatal("Whoami: expected SuccessResponse")
	}
	if whoami.SuccessResponse.ContentType != "text/plain" {
		t.Errorf("Whoami.SuccessResponse.ContentType = %q, want text/plain", whoami.SuccessResponse.ContentType)
	}
	if whoami.SuccessResponse.TypeName != "string" {
		t.Errorf("Whoami.SuccessResponse.TypeName = %q, want string", whoami.SuccessResponse.TypeName)
	}

	// GetVersion: text/plain with no schema — should default to string
	getVersion, ok := opMap["GetVersion"]
	if !ok {
		t.Fatal("expected GetVersion operation")
	}
	if getVersion.SuccessResponse == nil {
		t.Fatal("GetVersion: expected SuccessResponse")
	}
	if getVersion.SuccessResponse.ContentType != "text/plain" {
		t.Errorf("GetVersion.SuccessResponse.ContentType = %q, want text/plain", getVersion.SuccessResponse.ContentType)
	}
	if getVersion.SuccessResponse.TypeName != "string" {
		t.Errorf("GetVersion.SuccessResponse.TypeName = %q, want string", getVersion.SuccessResponse.TypeName)
	}

	// GetMixed: has both JSON and text/plain — should prefer JSON
	getMixed, ok := opMap["GetMixed"]
	if !ok {
		t.Fatal("expected GetMixed operation")
	}
	if getMixed.SuccessResponse == nil {
		t.Fatal("GetMixed: expected SuccessResponse")
	}
	if getMixed.SuccessResponse.ContentType != "application/json" {
		t.Errorf("GetMixed.SuccessResponse.ContentType = %q, want application/json", getMixed.SuccessResponse.ContentType)
	}
}

func TestAnalyzeOperations_EmptyPaths(t *testing.T) {
	a := New(&v3high.Document{})
	pkg, err := a.Analyze("test")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(pkg.Operations) != 0 {
		t.Errorf("expected no operations for nil paths, got %d", len(pkg.Operations))
	}
}

func TestMergeParams(t *testing.T) {
	boolTrue := true
	pathParams := []*v3high.Parameter{
		{Name: "petId", In: "path", Required: &boolTrue},
		{Name: "version", In: "header"},
	}
	opParams := []*v3high.Parameter{
		{Name: "petId", In: "path", Required: &boolTrue, Description: "overridden"},
	}

	merged := mergeParams(pathParams, opParams)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged params, got %d", len(merged))
	}

	// The first should be the path-level "version" (not overridden), second the op-level "petId".
	found := make(map[string]string)
	for _, p := range merged {
		found[p.Name] = p.Description
	}
	if _, ok := found["version"]; !ok {
		t.Error("expected version param in merged result")
	}
	if desc, ok := found["petId"]; !ok {
		t.Error("expected petId param in merged result")
	} else if desc != "overridden" {
		t.Errorf("petId should have overridden description, got %q", desc)
	}
}

func TestIsSuccessCode(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{"200", true},
		{"201", true},
		{"204", true},
		{"301", false},
		{"404", false},
		{"500", false},
		{"default", false},
	}
	for _, tt := range tests {
		if got := isSuccessCode(tt.code); got != tt.want {
			t.Errorf("isSuccessCode(%q) = %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestIsErrorCode(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{"400", true},
		{"404", true},
		{"500", true},
		{"200", false},
		{"301", false},
		{"default", false},
	}
	for _, tt := range tests {
		if got := isErrorCode(tt.code); got != tt.want {
			t.Errorf("isErrorCode(%q) = %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestPathToWords(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/users/{id}", "users id"},
		{"/pets/{petId}/toys", "pets petId toys"},
		{"/health", "health"},
	}
	for _, tt := range tests {
		if got := pathToWords(tt.path); got != tt.want {
			t.Errorf("pathToWords(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// TestDisambiguateParamNames_PositionalArgs covers path params colliding with
// the fixed positional arguments (ctx, body, and the params struct arg).
func TestDisambiguateParamNames_PositionalArgs(t *testing.T) {
	op := &ir.OperationDef{
		RequestBody: &ir.RequestBodyDef{},
		PathParams: []*ir.ParamDef{
			// Collides with the params struct argument (op has query params).
			{Name: "params", OrigName: "params", Location: "path", Required: true},
			// Collides with the fixed "body" argument (request body present).
			{Name: "body", OrigName: "body", Location: "path", Required: true},
			// Collides with the "result" local the method body declares.
			{Name: "result", OrigName: "result", Location: "path", Required: true},
		},
		QueryParams: []*ir.ParamDef{
			{Name: "limit", FieldName: "Limit", OrigName: "limit", Location: "query", Required: false},
		},
	}

	disambiguateParamNames(op)

	if got := op.PathParams[0].Name; got != "paramsPath" {
		t.Errorf("path param colliding with params arg: Name = %q, want %q", got, "paramsPath")
	}
	if got := op.PathParams[1].Name; got != "bodyPath" {
		t.Errorf("path param colliding with body arg: Name = %q, want %q", got, "bodyPath")
	}
	if got := op.PathParams[2].Name; got != "resultPath" {
		t.Errorf("path param colliding with the 'result' method local: Name = %q, want %q", got, "resultPath")
	}
	// Wire names must never change — only the Go identifier is disambiguated.
	if got := op.PathParams[0].OrigName; got != "params" {
		t.Errorf("OrigName changed to %q, want %q", got, "params")
	}
}

// TestDisambiguateParamNames_NoStructArg confirms "params"/"opts" are only
// reserved when the op actually has a params struct argument.
func TestDisambiguateParamNames_NoStructArg(t *testing.T) {
	op := &ir.OperationDef{
		PathParams: []*ir.ParamDef{
			{Name: "params", OrigName: "params", Location: "path", Required: true},
		},
	}
	disambiguateParamNames(op)
	if got := op.PathParams[0].Name; got != "params" {
		t.Errorf("path param Name = %q, want %q (no params struct arg to collide with)", got, "params")
	}
}

// TestDisambiguateParamNames_StructFields covers query/header/cookie params
// whose Go field names collide inside the shared params struct.
func TestDisambiguateParamNames_StructFields(t *testing.T) {
	op := &ir.OperationDef{
		QueryParams: []*ir.ParamDef{
			{Name: "userId", FieldName: "UserId", OrigName: "user-id", Location: "query", Required: true},
		},
		HeaderParams: []*ir.ParamDef{
			// PascalCases to the same "UserId" as the query param above.
			{Name: "userId", FieldName: "UserId", OrigName: "user_id", Location: "header", Required: true},
		},
		CookieParams: []*ir.ParamDef{
			{Name: "userId", FieldName: "UserId", OrigName: "userId", Location: "cookie", Required: false},
		},
	}

	disambiguateParamNames(op)

	if got := op.QueryParams[0].FieldName; got != "UserId" {
		t.Errorf("first field = %q, want %q (first occurrence keeps its name)", got, "UserId")
	}
	if got := op.HeaderParams[0].FieldName; got != "UserIdHeader" {
		t.Errorf("colliding header field = %q, want %q", got, "UserIdHeader")
	}
	if got := op.CookieParams[0].FieldName; got != "UserIdCookie" {
		t.Errorf("colliding cookie field = %q, want %q", got, "UserIdCookie")
	}
	// Wire names are untouched, so encoding still uses the spec names.
	if got := op.HeaderParams[0].OrigName; got != "user_id" {
		t.Errorf("OrigName changed to %q, want %q", got, "user_id")
	}
}

const headerDeclSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /things:
    get:
      operationId: listThings
      responses:
        "200":
          description: ok
          headers:
            X-Request-Id: { schema: { type: string } }
            X-Rate-Limit-Remaining: { schema: { type: integer } }
            X-Ratio: { schema: { type: number } }
            X-Deprecated: { schema: { type: boolean } }
            Last-Modified: { schema: { type: string, format: date-time } }
            X-Tags: { schema: { type: array, items: { type: string } } }
          content:
            application/json: { schema: { type: string } }
`

// A header arrives as text, so only the kinds text parses into unambiguously are
// typed. An HTTP date is not RFC 3339 and a list is comma-joined, so both stay
// the raw string rather than a type that would misread them.
func TestResponseHeaders_TypedOnlyWhereTextParses(t *testing.T) {
	pkg, _ := analyzeSpec(t, headerDeclSpec)

	var headers []*ir.ResponseHeaderDef
	for _, op := range pkg.Operations {
		for _, resp := range op.Responses {
			headers = append(headers, resp.Headers...)
		}
	}
	byName := make(map[string]*ir.ResponseHeaderDef, len(headers))
	for _, h := range headers {
		byName[h.Name] = h
	}
	if len(byName) != 6 {
		t.Fatalf("headers = %d, want 6", len(byName))
	}

	want := map[string]string{
		"X-Request-Id":           "string",
		"X-Rate-Limit-Remaining": "int64",
		"X-Ratio":                "float64",
		"X-Deprecated":           "bool",
		"Last-Modified":          "string",
		"X-Tags":                 "string",
	}
	for name, kind := range want {
		h := byName[name]
		if h == nil {
			t.Errorf("%s: not analyzed", name)
			continue
		}
		if h.Type != kind {
			t.Errorf("%s type = %q, want %q", name, h.Type, kind)
		}
	}
	if byName["X-Request-Id"].GoName != "XRequestID" {
		t.Errorf("X-Request-Id GoName = %q, want XRequestID", byName["X-Request-Id"].GoName)
	}
}

const multiMediaResponseSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /docs:
    get:
      operationId: getDoc
      responses:
        "200":
          description: ok
          content:
            application/xml: { schema: { type: string } }
            application/json: { schema: { type: string } }
  /one:
    get:
      operationId: getOne
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { type: string } }
`

// A response offering several media types still decodes one, and the author
// hears which rather than discovering it from the Accept header.
func TestMultiMediaResponse_PrefersJSONAndSaysSo(t *testing.T) {
	pkg, _ := analyzeSpec(t, multiMediaResponseSpec)

	for _, op := range pkg.Operations {
		if op.Name == "GetDoc" && op.SuccessResponse.ContentType != "application/json" {
			t.Errorf("GetDoc content type = %q, want application/json even though XML comes first", op.SuccessResponse.ContentType)
		}
	}

	var found int
	for _, w := range pkg.Warnings {
		if strings.Contains(w, "more than one media type") {
			found++
			if !strings.Contains(w, "1 response offers") {
				t.Errorf("warning = %q, want it to count the one response that offers several", w)
			}
		}
	}
	if found != 1 {
		t.Errorf("media type warnings = %d, want 1: %v", found, pkg.Warnings)
	}
}

const contentParamAnalyzerSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
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
        - name: legacy
          in: query
          content:
            application/xml:
              schema: { type: string }
        - name: plain
          in: query
          schema: { type: string }
      responses:
        "204": { description: ok }
components:
  schemas:
    Filter:
      type: object
      properties:
        field: { type: string }
`

// A parameter that names a media type carries a document, and the media type is
// what says how to serialize it.
func TestContentParam_CarriesItsMediaTypeAndSchema(t *testing.T) {
	pkg, _ := analyzeSpec(t, contentParamAnalyzerSpec)

	params := make(map[string]*ir.ParamDef)
	for _, op := range pkg.Operations {
		for _, p := range op.QueryParams {
			params[p.OrigName] = p
		}
	}

	filter := params["filter"]
	if filter == nil {
		t.Fatal("filter param not found")
	}
	if filter.ContentType != "application/json" {
		t.Errorf("filter ContentType = %q, want application/json", filter.ContentType)
	}
	if filter.Type != "*Filter" && filter.Type != "Filter" {
		t.Errorf("filter type = %q, want the declared schema rather than any", filter.Type)
	}

	// A media type the generator cannot serialize still types its value, and says
	// so rather than sending JSON where the server expects something else.
	if legacy := params["legacy"]; legacy == nil || legacy.ContentType != "application/xml" {
		t.Errorf("legacy param = %+v, want its media type carried", legacy)
	}
	if len(pkg.Warnings) != 1 || !strings.Contains(pkg.Warnings[0], "legacy") {
		t.Errorf("warnings = %v, want one naming the parameter that is not serialized", pkg.Warnings)
	}

	// A style-encoded parameter has no content type at all.
	if plain := params["plain"]; plain == nil || plain.ContentType != "" {
		t.Errorf("plain param = %+v, want no content type", plain)
	}
}

const allowReservedAnalyzerSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /lookup:
    get:
      operationId: lookup
      parameters:
        - { name: ref, in: query, allowReserved: true, schema: { type: string } }
        - { name: plain, in: query, schema: { type: string } }
      responses:
        "200":
          description: ok
          links:
            next: { operationId: lookup }
          content:
            application/json: { schema: { type: string } }
`

func TestAllowReserved_ReachesTheParam(t *testing.T) {
	pkg, _ := analyzeSpec(t, allowReservedAnalyzerSpec)

	params := make(map[string]*ir.ParamDef)
	for _, op := range pkg.Operations {
		for _, p := range op.QueryParams {
			params[p.OrigName] = p
		}
	}
	if ref := params["ref"]; ref == nil || !ref.AllowReserved {
		t.Errorf("ref param = %+v, want AllowReserved", ref)
	}
	if plain := params["plain"]; plain == nil || plain.AllowReserved {
		t.Errorf("plain param = %+v, want AllowReserved unset", plain)
	}
}

// Links are declared in bulk and the answer is the same for all of them, so the
// spec earns one warning rather than one per response.
func TestLinks_WarnOncePerSpec(t *testing.T) {
	pkg, _ := analyzeSpec(t, allowReservedAnalyzerSpec)

	var linkWarnings int
	for _, w := range pkg.Warnings {
		if strings.Contains(w, "links") {
			linkWarnings++
		}
	}
	if linkWarnings != 1 {
		t.Errorf("link warnings = %d, want 1: %v", linkWarnings, pkg.Warnings)
	}
}

const unionParamAnalyzerSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - name: either
          in: query
          schema:
            anyOf: [{ type: string }, { type: integer }]
        - name: same
          in: query
          schema:
            anyOf: [{ type: string }, { type: integer }]
        - name: maybe
          in: query
          schema:
            anyOf: [{ type: string }, { type: "null" }]
        - name: shaped
          in: query
          schema:
            type: object
            properties:
              lat: { type: number }
              lon: { type: number }
      responses:
        "204": { description: ok }
`

// A parameter with a genuine choice of types names one, which is what makes it
// constructible. A nullable union still collapses to its one variant.
func TestUnionParam_NamesItsType(t *testing.T) {
	pkg, typeMap := analyzeSpec(t, unionParamAnalyzerSpec)

	params := make(map[string]*ir.ParamDef)
	for _, op := range pkg.Operations {
		for _, p := range op.QueryParams {
			params[p.OrigName] = p
		}
	}

	either := params["either"]
	if either == nil || either.Type != "ListItemsEither" {
		t.Fatalf("either type = %+v, want a named union", either)
	}
	if td := typeMap[either.Type]; td == nil || td.Kind != ir.TypeKindUnion {
		t.Errorf("%s = %+v, want a generated union", either.Type, td)
	}
	// One shape is one type, so a second parameter of the same shape shares it.
	if same := params["same"]; same == nil || same.Type != either.Type {
		t.Errorf("same type = %+v, want %q", same, either.Type)
	}
	// Nullability is carried by the pointer, not by a union.
	if maybe := params["maybe"]; maybe == nil || maybe.Type != "string" {
		t.Errorf("maybe type = %+v, want string", maybe)
	}
	// An object written inline in a parameter is named for the same reason.
	if shaped := params["shaped"]; shaped == nil || shaped.Type != "ListItemsShaped" {
		t.Errorf("shaped type = %+v, want ListItemsShaped", shaped)
	}
}

const inlineMultipartAnalyzerSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /upload:
    post:
      operationId: upload
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file: { type: string, format: binary }
                meta:
                  type: object
                  properties:
                    thumbnail: { type: string, format: binary }
      responses:
        "204": { description: ok }
  /json:
    post:
      operationId: sendJSON
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                blob: { type: string, format: binary }
      responses:
        "204": { description: ok }
`

// Multipart is a property of where a schema is used, not of the schema, and an
// inline body had no name to record that under.
func TestInlineMultipart_BinaryPropertyBecomesAFilePart(t *testing.T) {
	_, typeMap := analyzeSpec(t, inlineMultipartAnalyzerSpec)

	body := typeMap["UploadBody"]
	if body == nil {
		t.Fatal("UploadBody not found")
	}
	fields := map[string]string{}
	for _, f := range body.Fields {
		fields[f.JSONName] = f.Type
	}
	// Optional here, so a pointer, but a file part either way.
	if fields["file"] != "*FormFile" {
		t.Errorf("file = %q, want *FormFile", fields["file"])
	}

	// A nested object is encoded as JSON, so bytes inside it stay bytes.
	nested := typeMap["UploadBodyMeta"]
	if nested == nil {
		t.Fatal("UploadBodyMeta not found")
	}
	if nested.Fields[0].Type != "[]byte" {
		t.Errorf("nested thumbnail = %q, want []byte", nested.Fields[0].Type)
	}

	// The same shape sent as JSON keeps byte slices throughout.
	jsonBody := typeMap["SendJSONBody"]
	if jsonBody == nil {
		t.Fatal("SendJSONBody not found")
	}
	if jsonBody.Fields[0].Type != "[]byte" {
		t.Errorf("json blob = %q, want []byte", jsonBody.Fields[0].Type)
	}
}

const collidingOperationSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /a-b:
    get:
      responses: { "204": { description: ok } }
  /a_b:
    get:
      responses: { "204": { description: ok } }
  /dup:
    get:
      operationId: sameName
      parameters:
        - { name: q, in: query, schema: { type: string } }
      responses: { "204": { description: ok } }
  /dup2:
    get:
      operationId: same-name
      parameters:
        - { name: q, in: query, schema: { type: string } }
      responses: { "204": { description: ok } }
`

// Two operations can derive one method name, from paths that differ only in
// punctuation or from operation ids that normalize together.
func TestOperationNames_CollisionsAreNumbered(t *testing.T) {
	pkg, typeMap := analyzeSpec(t, collidingOperationSpec)

	var names []string
	for _, op := range pkg.Operations {
		names = append(names, op.Name)
	}
	want := []string{"GetAB", "GetAB2", "SameName", "SameName2"}
	if len(names) != len(want) {
		t.Fatalf("operations = %v, want %v", names, want)
	}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("operation %d = %q, want %q", i, names[i], w)
		}
	}

	// The names params types are built from have to follow the method, or the
	// two would disagree about which operation they belong to.
	for _, name := range []string{"SameName", "SameName2"} {
		var op *ir.OperationDef
		for _, candidate := range pkg.Operations {
			if candidate.Name == name {
				op = candidate
			}
		}
		if op == nil {
			t.Fatalf("%s not found", name)
		}
		if len(op.QueryParams) != 1 {
			t.Errorf("%s params = %+v", name, op.QueryParams)
		}
	}
	_ = typeMap
}
