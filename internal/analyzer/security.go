package analyzer

import (
	"fmt"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
	"github.com/parallelworks/openapi-client-generator/internal/naming"
)

// analyzeSecuritySchemes walks Components.SecuritySchemes and populates pkg.AuthSchemes.
func (a *Analyzer) analyzeSecuritySchemes(pkg *ir.Package) error {
	if a.model.Components == nil || a.model.Components.SecuritySchemes == nil {
		return nil
	}

	for schemeName, schemeProxy := range a.model.Components.SecuritySchemes.FromOldest() {
		if schemeProxy == nil {
			continue
		}

		scheme := schemeProxy

		authScheme := &ir.AuthScheme{
			Name:        schemeName,
			GoName:      naming.ToGoName(schemeName),
			Description: scheme.Description,
		}

		switch scheme.Type {
		case "http":
			switch scheme.Scheme {
			case "bearer":
				authScheme.Type = ir.AuthTypeBearer
				authScheme.BearerFormat = scheme.BearerFormat
			case "basic":
				authScheme.Type = ir.AuthTypeBasic
			default:
				return fmt.Errorf("unsupported http scheme %q for security scheme %q", scheme.Scheme, schemeName)
			}

		case "apiKey":
			authScheme.Type = ir.AuthTypeAPIKey
			authScheme.APIKeyName = scheme.Name
			authScheme.APIKeyIn = scheme.In

		case "oauth2":
			authScheme.Type = ir.AuthTypeOAuth2
			if scheme.Flows != nil {
				authScheme.OAuthFlows = convertOAuthFlows(scheme.Flows)
			}

		default:
			return fmt.Errorf("unsupported security scheme type %q for %q", scheme.Type, schemeName)
		}

		pkg.AuthSchemes = append(pkg.AuthSchemes, authScheme)
	}

	return nil
}

// convertOAuthFlows converts libopenapi OAuthFlows to ir.OAuthFlowsDef.
func convertOAuthFlows(flows *v3high.OAuthFlows) *ir.OAuthFlowsDef {
	def := &ir.OAuthFlowsDef{}

	if flows.AuthorizationCode != nil {
		def.AuthorizationCode = convertOAuthFlow(flows.AuthorizationCode)
	}
	if flows.Implicit != nil {
		def.Implicit = convertOAuthFlow(flows.Implicit)
	}
	if flows.ClientCredentials != nil {
		def.ClientCredentials = convertOAuthFlow(flows.ClientCredentials)
	}
	if flows.Password != nil {
		def.Password = convertOAuthFlow(flows.Password)
	}

	return def
}

// convertOAuthFlow converts a single libopenapi OAuthFlow to ir.OAuthFlowDef.
func convertOAuthFlow(flow *v3high.OAuthFlow) *ir.OAuthFlowDef {
	def := &ir.OAuthFlowDef{
		AuthorizationURL: flow.AuthorizationUrl,
		TokenURL:         flow.TokenUrl,
		RefreshURL:       flow.RefreshUrl,
	}

	if flow.Scopes != nil {
		def.Scopes = make(map[string]string)
		for scopeName, scopeDesc := range flow.Scopes.FromOldest() {
			def.Scopes[scopeName] = scopeDesc
		}
	}

	return def
}
