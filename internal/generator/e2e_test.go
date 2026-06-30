package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/analyzer"
	"github.com/parallelworks/openapi-client-generator/internal/parser"
)

func projectRoot() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..")
}

func TestE2E_PetstoreGeneration(t *testing.T) {
	specPath := filepath.Join(projectRoot(), "testdata", "petstore.yaml")

	// Step 1: Parse the spec.
	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Step 2: Analyze (now includes operations).
	a := analyzer.New(result.Model)
	pkg, err := a.Analyze("petstore")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// Verify operations were analyzed.
	if len(pkg.Operations) != 4 {
		t.Fatalf("expected 4 operations, got %d", len(pkg.Operations))
	}

	// Verify types were analyzed.
	if len(pkg.Types) != 5 {
		t.Fatalf("expected 5 types, got %d", len(pkg.Types))
	}

	// Step 3: Generate all files.
	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New generator: %v", err)
	}

	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Step 4: Verify all expected files are produced.
	expectedFiles := map[string]bool{
		"types.go":      false,
		"client.go":     false,
		"options.go":    false,
		"helpers.go":    false,
		"operations.go": false,
		"pagination.go": false,
		"retry.go":      false,
		"middleware.go": false,
		"auth.go":       false,
		"errors.go":     false,
	}
	for _, f := range files {
		if _, ok := expectedFiles[f.Name]; ok {
			expectedFiles[f.Name] = true
		} else {
			t.Errorf("unexpected file generated: %s", f.Name)
		}
	}
	for name, found := range expectedFiles {
		if !found {
			t.Errorf("expected file not generated: %s", name)
		}
	}

	// Step 5: Write to temp dir and verify generated code compiles.
	tmpDir := t.TempDir()

	// Write go.mod
	goMod := []byte("module petstore-e2e-test\n\ngo 1.25.5\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	// Write all generated files.
	if err := WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}

	// List generated files for debugging.
	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		t.Logf("  generated: %s", e.Name())
	}

	// Run go build.
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Print each file for debugging.
		for _, f := range files {
			t.Logf("=== %s ===\n%s", f.Name, string(f.Content))
		}
		t.Fatalf("go build failed: %v\n%s", err, string(output))
	}
	t.Log("generated code compiles successfully")

	// Verify doc comments appear in generated output.
	var typesContent, opsContent string
	for _, f := range files {
		switch f.Name {
		case "types.go":
			typesContent = string(f.Content)
		case "operations.go":
			opsContent = string(f.Content)
		}
	}

	// Type-level doc comments.
	if !strings.Contains(typesContent, "// Pet - A pet in the store") {
		t.Error("types.go missing Pet type doc comment")
	}
	if !strings.Contains(typesContent, "// PetStatus - The current status of the pet in the store") {
		t.Error("types.go missing PetStatus type doc comment")
	}
	if !strings.Contains(typesContent, "// Error - An error response from the API") {
		t.Error("types.go missing Error type doc comment")
	}

	// Field-level doc comments.
	if !strings.Contains(typesContent, "// The unique identifier for the pet") {
		t.Error("types.go missing Pet.id field description")
	}
	if !strings.Contains(typesContent, "// The display name of the pet") {
		t.Error("types.go missing Pet.name field description")
	}
	if !strings.Contains(typesContent, "// A human-readable error message") {
		t.Error("types.go missing Error.message field description")
	}

	// Operation doc comments (summary + description).
	if !strings.Contains(opsContent, "// ListPets - List all pets") {
		t.Error("operations.go missing ListPets doc comment")
	}
	if !strings.Contains(opsContent, "// Returns a paginated list of all pets in the store.") {
		t.Error("operations.go missing ListPets description in doc comment")
	}
	if !strings.Contains(opsContent, "// CreatePet - Create a pet") {
		t.Error("operations.go missing CreatePet doc comment")
	}

	// Parameter descriptions in params struct.
	if !strings.Contains(opsContent, "// How many items to return at one time (max 100)") {
		t.Error("operations.go missing limit param description in ListPetsParams")
	}
	if !strings.Contains(opsContent, "// Pagination cursor") {
		t.Error("operations.go missing cursor param description in ListPetsParams")
	}
}

func TestE2E_TextPlainGeneration(t *testing.T) {
	specPath := filepath.Join(projectRoot(), "testdata", "text-plain.yaml")

	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	a := analyzer.New(result.Model)
	pkg, err := a.Analyze("textplain")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New generator: %v", err)
	}

	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Write to temp dir and verify compilation.
	tmpDir := t.TempDir()
	goMod := []byte("module textplain-e2e-test\n\ngo 1.25.5\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		for _, f := range files {
			t.Logf("=== %s ===\n%s", f.Name, string(f.Content))
		}
		t.Fatalf("go build failed: %v\n%s", err, string(output))
	}

	// Verify operations.go has correct content type handling.
	var opsContent string
	for _, f := range files {
		if f.Name == "operations.go" {
			opsContent = string(f.Content)
			break
		}
	}
	if opsContent == "" {
		t.Fatal("expected operations.go in generated files")
	}

	// Whoami should pass text/plain accept header.
	if !strings.Contains(opsContent, `"text/plain"`) {
		t.Error("operations.go should contain text/plain accept header for Whoami")
	}

	// HealthCheck should pass application/json accept header.
	if !strings.Contains(opsContent, `"application/json"`) {
		t.Error("operations.go should contain application/json accept header for HealthCheck")
	}

	// Verify client.go uses accept parameter and has content-type-aware decoding.
	var clientContent string
	for _, f := range files {
		if f.Name == "client.go" {
			clientContent = string(f.Content)
			break
		}
	}
	if clientContent == "" {
		t.Fatal("expected client.go in generated files")
	}

	if !strings.Contains(clientContent, "accept string") {
		t.Error("client.go do() method should have accept string parameter")
	}
	if !strings.Contains(clientContent, `resp.Header.Get("Content-Type")`) {
		t.Error("client.go should check response Content-Type header")
	}
	if !strings.Contains(clientContent, `result.(*string)`) {
		t.Error("client.go should have *string type assertion for text responses")
	}

	t.Log("text/plain generated code compiles successfully")
}

func TestE2E_HeaderParamsGeneration(t *testing.T) {
	specPath := filepath.Join(projectRoot(), "testdata", "header-params.yaml")

	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	a := analyzer.New(result.Model)
	pkg, err := a.Analyze("headertest")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if len(pkg.Operations) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(pkg.Operations))
	}

	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New generator: %v", err)
	}

	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Write to temp dir and verify compilation.
	tmpDir := t.TempDir()
	goMod := []byte("module headertest-e2e\n\ngo 1.25.5\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		for _, f := range files {
			t.Logf("=== %s ===\n%s", f.Name, string(f.Content))
		}
		t.Fatalf("go build failed: %v\n%s", err, string(output))
	}

	// Verify operations.go content for the three cases.
	var opsContent string
	for _, f := range files {
		if f.Name == "operations.go" {
			opsContent = string(f.Content)
			break
		}
	}
	if opsContent == "" {
		t.Fatal("expected operations.go in generated files")
	}

	// Case 1: Header-only params — should have headers but no queryValues.
	if !strings.Contains(opsContent, "CreateChatCompletionParams") {
		t.Error("missing CreateChatCompletionParams struct")
	}
	if !strings.Contains(opsContent, `setHeader(headers, "X-Conversation-Id"`) {
		t.Error("missing header set for X-Conversation-Id in CreateChatCompletion")
	}
	if !strings.Contains(opsContent, `setHeader(headers, "X-Request-Priority"`) {
		t.Error("missing header set for X-Request-Priority in CreateChatCompletion")
	}

	// Case 2: Mixed query + header params — should have both queryValues and headers.
	if !strings.Contains(opsContent, "StreamChatCompletionParams") {
		t.Error("missing StreamChatCompletionParams struct")
	}
	if !strings.Contains(opsContent, `addQueryParam(queryValues, "limit"`) {
		t.Error("missing query param for limit in StreamChatCompletion")
	}
	if !strings.Contains(opsContent, `setHeader(headers, "X-Conversation-Id"`) {
		t.Error("missing header set for X-Conversation-Id in StreamChatCompletion")
	}

	// Case 3: No optional params — should have no opts or headers.
	if strings.Contains(opsContent, "HealthCheckParams") {
		t.Error("HealthCheck should not have a params struct")
	}

	t.Log("header params generated code compiles successfully")
}

func TestE2E_AddQueryParamPointerTypes(t *testing.T) {
	specPath := filepath.Join(projectRoot(), "testdata", "petstore.yaml")

	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	a := analyzer.New(result.Model)
	pkg, err := a.Analyze("petstore")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New generator: %v", err)
	}

	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	tmpDir := t.TempDir()

	goMod := []byte("module petstore-queryparams-test\n\ngo 1.25.5\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	if err := WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}

	// Write a test file that exercises addQueryParam with pointer types.
	testCode := []byte(`package petstore

import (
	"net/url"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestAddQueryParam_PointerString(t *testing.T) {
	values := url.Values{}
	addQueryParam(values, "name", ptr("hello"))
	if got := values.Get("name"); got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

func TestAddQueryParam_PointerInt(t *testing.T) {
	values := url.Values{}
	addQueryParam(values, "limit", ptr(int32(42)))
	if got := values.Get("limit"); got != "42" {
		t.Errorf("expected %q, got %q", "42", got)
	}
}

func TestAddQueryParam_NilStringPointer(t *testing.T) {
	values := url.Values{}
	var s *string
	addQueryParam(values, "name", s)
	if values.Has("name") {
		t.Errorf("expected no param, got %q", values.Get("name"))
	}
}

func TestAddQueryParam_NilIntPointer(t *testing.T) {
	values := url.Values{}
	var n *int32
	addQueryParam(values, "limit", n)
	if values.Has("limit") {
		t.Errorf("expected no param, got %q", values.Get("limit"))
	}
}

func TestAddQueryParam_UntypedNil(t *testing.T) {
	values := url.Values{}
	addQueryParam(values, "key", nil)
	if values.Has("key") {
		t.Errorf("expected no param, got %q", values.Get("key"))
	}
}

func TestAddQueryParam_NonPointerString(t *testing.T) {
	values := url.Values{}
	addQueryParam(values, "name", "world")
	if got := values.Get("name"); got != "world" {
		t.Errorf("expected %q, got %q", "world", got)
	}
}

func TestAddQueryParam_NonPointerInt(t *testing.T) {
	values := url.Values{}
	addQueryParam(values, "count", int32(7))
	if got := values.Get("count"); got != "7" {
		t.Errorf("expected %q, got %q", "7", got)
	}
}
`)
	if err := os.WriteFile(filepath.Join(tmpDir, "helpers_test.go"), testCode, 0o644); err != nil {
		t.Fatalf("writing helpers_test.go: %v", err)
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		for _, f := range files {
			t.Logf("=== %s ===\n%s", f.Name, string(f.Content))
		}
		t.Fatalf("go test failed: %v\n%s", err, string(output))
	}
	t.Logf("addQueryParam pointer tests passed:\n%s", string(output))
}

// TestE2E_CollidingParamNames verifies the two disambiguation namespaces: a
// path param whose Go name collides with the "params" struct argument, and two
// params (query + header) whose Go field names collide inside the params struct.
// Both must be renamed so the generated code compiles.
func TestE2E_CollidingParamNames(t *testing.T) {
	spec := `openapi: 3.0.0
info:
  title: Collision
  version: 1.0.0
paths:
  /things/{params}:
    get:
      operationId: getThing
      parameters:
        - name: params
          in: path
          required: true
          schema:
            type: string
        - name: user-id
          in: query
          required: true
          schema:
            type: string
        - name: user_id
          in: header
          required: true
          schema:
            type: string
      responses:
        '200':
          description: ok
`
	specPath := filepath.Join(t.TempDir(), "collision.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("writing spec: %v", err)
	}

	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	pkg, err := analyzer.New(result.Model).Analyze("collision")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var ops string
	for _, f := range files {
		if f.Name == "operations.go" {
			ops = string(f.Content)
		}
	}
	// The path param colliding with the params struct arg is renamed; its wire
	// name (used by pathReplace) is untouched.
	if !strings.Contains(ops, "paramsPath string") {
		t.Errorf("path param colliding with params arg not disambiguated:\n%s", ops)
	}
	if !strings.Contains(ops, `pathReplace(path, "params", paramsPath)`) {
		t.Errorf("disambiguated path param must still substitute under its wire name:\n%s", ops)
	}
	// The two same-FieldName params must get distinct struct fields, each still
	// encoded under its own wire name.
	if !strings.Contains(ops, `addQueryParam(queryValues, "user-id", params.UserID)`) {
		t.Errorf("query field not at UserID:\n%s", ops)
	}
	if !strings.Contains(ops, `setHeader(headers, "user_id", params.UserIDHeader)`) {
		t.Errorf("colliding header field not disambiguated to UserIDHeader:\n%s", ops)
	}

	tmpDir := t.TempDir()
	goMod := []byte("module collision-e2e-test\n\ngo 1.25.5\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		for _, f := range files {
			t.Logf("=== %s ===\n%s", f.Name, string(f.Content))
		}
		t.Fatalf("generated code with colliding param names failed to compile: %v\n%s", err, string(output))
	}
}

// TestE2E_PaginatedWithRequiredQueryParam guards against the *Iter wrapper
// dropping required query params. A paginated operation that also has a
// required query filter must thread that filter through both the Iter signature
// and the underlying method call, or the generated package fails to compile.
func TestE2E_PaginatedWithRequiredQueryParam(t *testing.T) {
	spec := `openapi: 3.0.0
info:
  title: Paged
  version: 1.0.0
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - name: status
          in: query
          required: true
          schema:
            type: string
        - name: cursor
          in: query
          required: false
          schema:
            type: string
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ItemList'
components:
  schemas:
    ItemList:
      type: object
      properties:
        items:
          type: array
          items:
            type: string
        next_cursor:
          type: string
`
	specPath := filepath.Join(t.TempDir(), "paged.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("writing spec: %v", err)
	}

	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	pkg, err := analyzer.New(result.Model).Analyze("paged")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var pag string
	for _, f := range files {
		if f.Name == "pagination.go" {
			pag = string(f.Content)
		}
	}
	if pag == "" {
		t.Fatal("pagination.go not generated — spec was not detected as paginated")
	}
	// The required query param makes the params struct mandatory; the Iter must
	// take it and forward it (as p) to the underlying call.
	if !strings.Contains(pag, "params ListItemsParams)") {
		t.Errorf("Iter method missing mandatory params struct in signature:\n%s", pag)
	}
	if !strings.Contains(pag, "p := params") {
		t.Errorf("Iter does not seed the page params from the required struct:\n%s", pag)
	}
	if !strings.Contains(pag, "c.ListItems(ctx, p)") {
		t.Errorf("Iter call site does not forward the params struct:\n%s", pag)
	}

	tmpDir := t.TempDir()
	goMod := []byte("module paged-e2e-test\n\ngo 1.25.5\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		for _, f := range files {
			t.Logf("=== %s ===\n%s", f.Name, string(f.Content))
		}
		t.Fatalf("paginated client with a required query param failed to compile: %v\n%s", err, string(output))
	}
}

// TestE2E_RequiredHeadersAndCookies verifies that required header and cookie
// params are sent on the wire (previously they were silently dropped), optional
// ones are conditional, and the generated client actually transmits them — using
// a real HTTP round-trip against an httptest server.
func TestE2E_RequiredHeadersAndCookies(t *testing.T) {
	spec := `openapi: 3.0.0
info:
  title: Hdr
  version: 1.0.0
paths:
  /x:
    get:
      operationId: getX
      parameters:
        - name: version
          in: header
          required: true
          schema:
            type: string
        - name: trace
          in: header
          required: false
          schema:
            type: string
        - name: session
          in: cookie
          required: true
          schema:
            type: string
        - name: theme
          in: cookie
          required: false
          schema:
            type: string
      responses:
        '200':
          description: ok
`
	specPath := filepath.Join(t.TempDir(), "hdr.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("writing spec: %v", err)
	}
	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	pkg, err := analyzer.New(result.Model).Analyze("hdr")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var ops string
	for _, f := range files {
		if f.Name == "operations.go" {
			ops = string(f.Content)
		}
	}
	// Required header set unconditionally; required cookie always added.
	if !strings.Contains(ops, `setHeader(headers, "version", params.Version)`) {
		t.Errorf("required header not set unconditionally:\n%s", ops)
	}
	if !strings.Contains(ops, `addCookieHeader(headers, "session", params.Session)`) {
		t.Errorf("required cookie not sent:\n%s", ops)
	}
	if !strings.Contains(ops, `addCookieHeader(headers, "theme", params.Theme)`) {
		t.Errorf("optional cookie not wired:\n%s", ops)
	}
	if !strings.Contains(ops, "params GetXParams)") {
		t.Errorf("required header/cookie did not make the params struct mandatory:\n%s", ops)
	}

	tmpDir := t.TempDir()
	goMod := []byte("module hdr-e2e-test\n\ngo 1.25.5\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}
	// A generated round-trip test: the client must actually transmit the required
	// header + cookie, send the optional header, and omit the absent optional cookie.
	roundTrip := []byte(`package hdr

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequiredHeadersAndCookiesOnTheWire(t *testing.T) {
	var gotVersion, gotTrace, gotCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get("version")
		gotTrace = r.Header.Get("trace")
		gotCookie = r.Header.Get("Cookie")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	trace := "tr-1"
	// The session value contains a ';' — a raw concatenation would split the
	// cookie and corrupt the header; http.Cookie sanitizes the invalid byte away.
	if err := NewClient(srv.URL).GetX(t.Context(), GetXParams{
		Version: "v2",
		Trace:   &trace,
		Session: "ab;cd",
		Theme:   nil,
	}); err != nil {
		t.Fatalf("GetX: %v", err)
	}
	if gotVersion != "v2" {
		t.Errorf("required header version = %q, want v2", gotVersion)
	}
	if gotTrace != "tr-1" {
		t.Errorf("optional header trace = %q, want tr-1", gotTrace)
	}
	if gotCookie != "session=abcd" {
		t.Errorf("Cookie = %q, want session=abcd (required sent + ';' sanitized, optional theme omitted)", gotCookie)
	}
}
`)
	if err := os.WriteFile(filepath.Join(tmpDir, "wire_test.go"), roundTrip, 0o644); err != nil {
		t.Fatalf("writing wire_test.go: %v", err)
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		for _, f := range files {
			t.Logf("=== %s ===\n%s", f.Name, string(f.Content))
		}
		t.Fatalf("required header/cookie round-trip failed: %v\n%s", err, string(output))
	}
}

// TestE2E_BodyWithRequiredParam covers an operation that has BOTH a request body
// and a required param: the signature must be (ctx, body, params) and compile.
func TestE2E_BodyWithRequiredParam(t *testing.T) {
	spec := `openapi: 3.0.0
info:
  title: Body
  version: 1.0.0
paths:
  /things:
    post:
      operationId: createThing
      parameters:
        - name: idempotency-key
          in: header
          required: true
          schema:
            type: string
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Thing'
      responses:
        '201':
          description: created
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Thing'
components:
  schemas:
    Thing:
      type: object
      properties:
        name:
          type: string
`
	files, ops := generateFromSpec(t, spec, "body")
	if !strings.Contains(ops, "body Thing, params CreateThingParams)") {
		t.Errorf("body + required param signature wrong (want ctx, body, params):\n%s", ops)
	}
	if !strings.Contains(ops, `setHeader(headers, "idempotency-key", params.IdempotencyKey)`) {
		t.Errorf("required header not set from the params struct:\n%s", ops)
	}
	buildGenerated(t, files, "body-e2e-test")
}

// TestE2E_RequiredCursorNotPaginated guards the required-cursor edge: a required
// cursor can't be driven by the iterator (its field is a value, not a pointer),
// so the op must not be paginated and the package must still compile.
func TestE2E_RequiredCursorNotPaginated(t *testing.T) {
	spec := `openapi: 3.0.0
info:
  title: ReqCursor
  version: 1.0.0
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - name: cursor
          in: query
          required: true
          schema:
            type: string
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ItemList'
components:
  schemas:
    ItemList:
      type: object
      properties:
        items:
          type: array
          items:
            type: string
        next_cursor:
          type: string
`
	files, _ := generateFromSpec(t, spec, "reqcursor")
	var pag string
	for _, f := range files {
		if f.Name == "pagination.go" {
			pag = string(f.Content)
		}
	}
	if strings.Contains(pag, "ListItemsIter") {
		t.Errorf("required cursor must not generate an iterator (would not compile):\n%s", pag)
	}
	buildGenerated(t, files, "reqcursor-e2e-test")
}

// TestE2E_PathParamShadowsImport guards against a path param whose Go name
// matches a package the generated method body uses (url/http/fmt/context). Such a
// name must be suffixed, or it shadows the import (`url.Values{}` -> "url.Values
// is not a type") and the package fails to compile.
func TestE2E_PathParamShadowsImport(t *testing.T) {
	spec := `openapi: 3.0.0
info:
  title: Shadow
  version: 1.0.0
paths:
  /proxy/{url}:
    get:
      operationId: getProxy
      parameters:
        - name: url
          in: path
          required: true
          schema:
            type: string
        - name: ttl
          in: query
          required: false
          schema:
            type: integer
      responses:
        '200':
          description: ok
`
	files, ops := generateFromSpec(t, spec, "shadow")
	// The path param is renamed off the import, but still substitutes under its
	// original wire name.
	if !strings.Contains(ops, "urlPath string") {
		t.Errorf("path param shadowing the net/url import was not disambiguated:\n%s", ops)
	}
	if !strings.Contains(ops, `pathReplace(path, "url", urlPath)`) {
		t.Errorf("disambiguated path param must still substitute under its wire name:\n%s", ops)
	}
	buildGenerated(t, files, "shadow-e2e-test")
}

// TestE2E_PaginatedWithBody guards the *Iter wrapper forwarding a request body. A
// paginated operation that also has a body must thread it through the Iter
// signature and the underlying call, or the generated package fails to compile.
func TestE2E_PaginatedWithBody(t *testing.T) {
	spec := `openapi: 3.0.0
info:
  title: PagedBody
  version: 1.0.0
paths:
  /search:
    post:
      operationId: search
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Query'
      parameters:
        - name: cursor
          in: query
          required: false
          schema:
            type: string
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ItemList'
components:
  schemas:
    Query:
      type: object
      properties:
        term:
          type: string
    ItemList:
      type: object
      properties:
        items:
          type: array
          items:
            type: string
        next_cursor:
          type: string
`
	files, _ := generateFromSpec(t, spec, "pagedbody")
	var pag string
	for _, f := range files {
		if f.Name == "pagination.go" {
			pag = string(f.Content)
		}
	}
	if pag == "" {
		t.Fatal("pagination.go not generated — spec was not detected as paginated")
	}
	// The Iter must accept and forward the body, between the path params and the
	// page-params struct, matching the underlying method's argument order.
	if !strings.Contains(pag, "body Query") {
		t.Errorf("Iter signature does not accept the request body:\n%s", pag)
	}
	if !strings.Contains(pag, "c.Search(ctx, body, p)") {
		t.Errorf("Iter call site does not forward the body:\n%s", pag)
	}
	buildGenerated(t, files, "pagedbody-e2e-test")
}

// TestE2E_ParamEncodingOnTheWire is a round-trip covering three encodings the
// generator previously got wrong: a []byte (format:byte) param sent as its string
// value (not "[104 105]"), an explicitly-set optional zero value (?flag=false)
// reaching the server, and an array header comma-joined (X-Tags: a,b).
func TestE2E_ParamEncodingOnTheWire(t *testing.T) {
	spec := `openapi: 3.0.0
info:
  title: Wire
  version: 1.0.0
paths:
  /x:
    get:
      operationId: getX
      parameters:
        - name: sig
          in: query
          required: true
          schema:
            type: string
            format: byte
        - name: flag
          in: query
          required: false
          schema:
            type: boolean
        - name: X-Tags
          in: header
          required: true
          schema:
            type: array
            items:
              type: string
      responses:
        '200':
          description: ok
`
	files, ops := generateFromSpec(t, spec, "wire")
	if !strings.Contains(ops, "Sig []byte") {
		t.Errorf("format:byte query param should be a []byte field:\n%s", ops)
	}

	tmpDir := t.TempDir()
	goMod := []byte("module wire-e2e-test\n\ngo 1.25.5\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}
	roundTrip := []byte(`package wire

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestParamEncodingOnTheWire(t *testing.T) {
	var gotSig, gotFlag, gotTags string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.URL.Query().Get("sig")
		gotFlag = r.URL.Query().Get("flag")
		gotTags = r.Header.Get("X-Tags")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := NewClient(srv.URL).GetX(t.Context(), GetXParams{
		Sig:   []byte("hi"),
		Flag:  ptr(false),
		XTags: []string{"a", "b"},
	}); err != nil {
		t.Fatalf("GetX: %v", err)
	}
	if gotSig != "hi" {
		t.Errorf("[]byte query param sig = %q, want \"hi\" (not Go byte-slice notation)", gotSig)
	}
	if gotFlag != "false" {
		t.Errorf("explicitly-set optional flag = %q, want \"false\" (zero value must be sent)", gotFlag)
	}
	if gotTags != "a,b" {
		t.Errorf("array header X-Tags = %q, want \"a,b\" (comma-joined)", gotTags)
	}
}
`)
	if err := os.WriteFile(filepath.Join(tmpDir, "wire_test.go"), roundTrip, 0o644); err != nil {
		t.Fatalf("writing wire_test.go: %v", err)
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		for _, f := range files {
			t.Logf("=== %s ===\n%s", f.Name, string(f.Content))
		}
		t.Fatalf("param-encoding round-trip failed: %v\n%s", err, string(output))
	}
}

// generateFromSpec parses an inline spec, analyzes, generates, and returns the
// files plus the operations.go content.
func generateFromSpec(t *testing.T, spec, pkgName string) ([]GeneratedFile, string) {
	t.Helper()
	specPath := filepath.Join(t.TempDir(), "spec.yaml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("writing spec: %v", err)
	}
	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	pkg, err := analyzer.New(result.Model).Analyze(pkgName)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var ops string
	for _, f := range files {
		if f.Name == "operations.go" {
			ops = string(f.Content)
		}
	}
	return files, ops
}

// buildGenerated writes files to a temp module and runs `go build ./...`.
func buildGenerated(t *testing.T, files []GeneratedFile, module string) {
	t.Helper()
	tmpDir := t.TempDir()
	goMod := []byte("module " + module + "\n\ngo 1.25.5\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		for _, f := range files {
			t.Logf("=== %s ===\n%s", f.Name, string(f.Content))
		}
		t.Fatalf("generated code failed to compile: %v\n%s", err, string(output))
	}
}
