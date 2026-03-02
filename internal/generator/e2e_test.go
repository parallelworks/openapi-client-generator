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
