package parser

import (
	"path/filepath"
	"runtime"
	"testing"
)

func projectRoot() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..")
}

func TestParsePetstore(t *testing.T) {
	specPath := filepath.Join(projectRoot(), "testdata", "petstore.yaml")

	result, err := Parse(specPath, Config{})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if result.Model == nil {
		t.Fatal("expected non-nil Model")
	}

	if result.Version != "3.1.0" {
		t.Errorf("expected version 3.1.0, got %s", result.Version)
	}

	if result.Model.Components == nil {
		t.Fatal("expected non-nil Components")
	}

	if result.Model.Components.Schemas == nil {
		t.Fatal("expected non-nil Schemas")
	}

	schemaCount := result.Model.Components.Schemas.Len()
	if schemaCount == 0 {
		t.Error("expected at least one schema in Components")
	}

	t.Logf("Parsed OpenAPI %s with %d schemas", result.Version, schemaCount)
}

func TestParseNonExistentFile(t *testing.T) {
	_, err := Parse("/nonexistent/path.yaml", Config{})
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}
