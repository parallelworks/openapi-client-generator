package analyzer

import (
	"path/filepath"
	"testing"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
	"github.com/parallelworks/openapi-client-generator/internal/parser"
)

func TestAnalyzeSecuritySchemes_Petstore(t *testing.T) {
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

	if len(pkg.AuthSchemes) != 2 {
		t.Fatalf("expected 2 auth schemes, got %d", len(pkg.AuthSchemes))
	}

	schemeMap := make(map[string]*ir.AuthScheme)
	for _, s := range pkg.AuthSchemes {
		schemeMap[s.Name] = s
		t.Logf("AuthScheme: %s (type=%d, goName=%s)", s.Name, s.Type, s.GoName)
	}

	// Verify bearerAuth scheme.
	bearer, ok := schemeMap["bearerAuth"]
	if !ok {
		t.Fatal("expected bearerAuth scheme")
	}
	if bearer.Type != ir.AuthTypeBearer {
		t.Errorf("bearerAuth.Type = %d, want %d (AuthTypeBearer)", bearer.Type, ir.AuthTypeBearer)
	}
	if bearer.GoName != "BearerAuth" {
		t.Errorf("bearerAuth.GoName = %q, want BearerAuth", bearer.GoName)
	}

	// Verify apiKeyAuth scheme.
	apiKey, ok := schemeMap["apiKeyAuth"]
	if !ok {
		t.Fatal("expected apiKeyAuth scheme")
	}
	if apiKey.Type != ir.AuthTypeAPIKey {
		t.Errorf("apiKeyAuth.Type = %d, want %d (AuthTypeAPIKey)", apiKey.Type, ir.AuthTypeAPIKey)
	}
	if apiKey.GoName != "APIKeyAuth" {
		t.Errorf("apiKeyAuth.GoName = %q, want APIKeyAuth", apiKey.GoName)
	}
	if apiKey.APIKeyName != "X-API-Key" {
		t.Errorf("apiKeyAuth.APIKeyName = %q, want X-API-Key", apiKey.APIKeyName)
	}
	if apiKey.APIKeyIn != "header" {
		t.Errorf("apiKeyAuth.APIKeyIn = %q, want header", apiKey.APIKeyIn)
	}
}

func TestAnalyzeSecuritySchemes_Empty(t *testing.T) {
	a := New(&v3high.Document{})
	pkg, err := a.Analyze("test")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(pkg.AuthSchemes) != 0 {
		t.Errorf("expected no auth schemes for empty doc, got %d", len(pkg.AuthSchemes))
	}
}

func TestAnalyzeSecuritySchemes_NilSecuritySchemes(t *testing.T) {
	result, err := parser.Parse(filepath.Join(projectRoot(), "testdata", "petstore.yaml"), parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Nil out security schemes to test the empty path.
	origSchemes := result.Model.Components.SecuritySchemes
	result.Model.Components.SecuritySchemes = nil

	a := New(result.Model)
	pkg, err := a.Analyze("test")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(pkg.AuthSchemes) != 0 {
		t.Errorf("expected no auth schemes for nil security schemes, got %d", len(pkg.AuthSchemes))
	}

	result.Model.Components.SecuritySchemes = origSchemes
}
