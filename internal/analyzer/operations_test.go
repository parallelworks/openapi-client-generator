package analyzer

import (
	"path/filepath"
	"testing"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
	"github.com/parallelworks/openapi-client-generator/internal/parser"
)

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
