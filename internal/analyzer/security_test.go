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

const degradedSecuritySpec = `openapi: 3.1.0
info: { title: sec, version: "1" }
paths:
  /t:
    get:
      operationId: getT
      responses:
        "204": { description: ok }
components:
  securitySchemes:
    oidc:
      type: openIdConnect
      openIdConnectUrl: https://issuer.example.com/.well-known/openid-configuration
    caps:
      type: http
      scheme: Bearer
    legacy:
      type: http
      scheme: digest
    mtls:
      type: mutualTLS
    weird:
      type: somethingNew
`

// A scheme the generator cannot act on is one declaration in a spec the caller
// may not even authenticate with, so it costs a warning rather than the client.
func TestSecuritySchemes_UnsupportedDegradeToAWarning(t *testing.T) {
	pkg, _ := analyzeSpec(t, degradedSecuritySpec)

	byName := make(map[string]*ir.AuthScheme, len(pkg.AuthSchemes))
	for _, s := range pkg.AuthSchemes {
		byName[s.Name] = s
	}

	// OpenID Connect is a bearer token on the wire.
	if oidc := byName["oidc"]; oidc == nil || oidc.Type != ir.AuthTypeBearer {
		t.Errorf("oidc scheme = %+v, want a bearer provider", oidc)
	}
	// RFC 7235 makes the scheme name case-insensitive.
	if caps := byName["caps"]; caps == nil || caps.Type != ir.AuthTypeBearer {
		t.Errorf("caps scheme = %+v, want a bearer provider", caps)
	}
	for _, name := range []string{"legacy", "mtls", "weird"} {
		if s := byName[name]; s != nil {
			t.Errorf("%s scheme = %+v, want no provider", name, s)
		}
	}
	if len(pkg.Warnings) != 3 {
		t.Errorf("warnings = %v, want one each for legacy, mtls, and weird", pkg.Warnings)
	}
}
