package analyzer

import (
	"fmt"
	"strings"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	naming "github.com/giraffesyo/openapi-go-naming"
	"github.com/parallelworks/openapi-client-generator/internal/ir"
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
			GoName:      naming.Exported(schemeName),
			Description: scheme.Description,
		}

		switch scheme.Type {
		case "http":
			// RFC 7235 registers auth scheme names case-insensitively, and specs
			// spell this one both ways.
			switch strings.ToLower(scheme.Scheme) {
			case "bearer":
				authScheme.Type = ir.AuthTypeBearer
				authScheme.BearerFormat = scheme.BearerFormat
			case "basic":
				authScheme.Type = ir.AuthTypeBasic
			default:
				pkg.Warnings = append(pkg.Warnings, fmt.Sprintf("security scheme %q: no provider generated for http scheme %q", schemeName, scheme.Scheme))
				continue
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

		case "openIdConnect":
			// OpenID Connect is a bearer token on the wire; the discovery document
			// is the caller's business, not the transport's.
			authScheme.Type = ir.AuthTypeBearer

		case "mutualTLS":
			// The certificate is configured on the transport, so there is nothing
			// for a provider to add to the request.
			pkg.Warnings = append(pkg.Warnings, fmt.Sprintf("security scheme %q: mutualTLS is configured on the HTTP transport, so no provider is generated", schemeName))
			continue

		default:
			pkg.Warnings = append(pkg.Warnings, fmt.Sprintf("security scheme %q: no provider generated for unrecognized type %q", schemeName, scheme.Type))
			continue
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
