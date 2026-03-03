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
		"middleware.go":  false,
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
	if !strings.Contains(opsContent, `headers.Set("X-Conversation-Id"`) {
		t.Error("missing header set for X-Conversation-Id in CreateChatCompletion")
	}
	if !strings.Contains(opsContent, `headers.Set("X-Request-Priority"`) {
		t.Error("missing header set for X-Request-Priority in CreateChatCompletion")
	}

	// Case 2: Mixed query + header params — should have both queryValues and headers.
	if !strings.Contains(opsContent, "StreamChatCompletionParams") {
		t.Error("missing StreamChatCompletionParams struct")
	}
	if !strings.Contains(opsContent, `addQueryParam(queryValues, "limit"`) {
		t.Error("missing query param for limit in StreamChatCompletion")
	}
	if !strings.Contains(opsContent, `headers.Set("X-Conversation-Id"`) {
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
