package ir

// AuthType represents the type of authentication.
type AuthType int

const (
	AuthTypeAPIKey AuthType = iota
	AuthTypeBearer
	AuthTypeBasic
	AuthTypeOAuth2
)

// AuthScheme represents a security scheme from the spec.
type AuthScheme struct {
	Name         string   // Identifier from the spec
	GoName       string   // Go identifier
	Type         AuthType
	Description  string
	// For APIKey:
	APIKeyName string // Header/query parameter name
	APIKeyIn   string // "header", "query", "cookie"
	// For HTTP Bearer:
	BearerFormat string
	// For OAuth2:
	OAuthFlows *OAuthFlowsDef
}

// OAuthFlowsDef describes OAuth2 flows.
type OAuthFlowsDef struct {
	AuthorizationCode *OAuthFlowDef
	Implicit          *OAuthFlowDef
	ClientCredentials *OAuthFlowDef
	Password          *OAuthFlowDef
}

// OAuthFlowDef describes a single OAuth2 flow.
type OAuthFlowDef struct {
	AuthorizationURL string
	TokenURL         string
	RefreshURL       string
	Scopes           map[string]string // scope name -> description
}
