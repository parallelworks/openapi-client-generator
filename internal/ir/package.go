package ir

// Package is the top-level IR representing the entire generated package.
type Package struct {
	Name        string          // Go package name
	Types       []*TypeDef      // All type definitions
	Operations  []*OperationDef // All API operations
	AuthSchemes []*AuthScheme   // Security schemes from the spec
	Servers     []*ServerDef    // Servers the spec declares, in spec order
	Info        *APIInfo        // API title, version, description
	UserAgent   string          // Default User-Agent for generated clients
	Warnings    []string        // Spec constructs the generator could not act on
	Webhooks    []*WebhookDef   // Inbound payloads: webhooks and callbacks
}

// WebhookDef is one payload the API sends rather than receives: a webhook the
// document declares, or a callback an operation registers.
type WebhookDef struct {
	Name        string // Dispatch key: the webhook's name, or operation.callback
	GoName      string // Identifier fragment: Parse<GoName>Webhook
	Callback    bool   // Whether this came from an operation's callbacks
	Method      string // HTTP method the sender uses
	PayloadType string // Go type the body decodes into
	Description string
}

// ServerDef is one entry of the spec's servers list.
type ServerDef struct {
	URL         string
	Description string
	Variables   []*ServerVar // Template variables, in the order they appear in URL
}

// ServerVar is one template variable of a server URL.
type ServerVar struct {
	Name        string // Name as it appears in the URL template
	GoName      string // Go parameter name
	Default     string
	Enum        []string
	Description string
}

// APIInfo contains metadata about the API.
type APIInfo struct {
	Title       string
	Version     string
	Description string
}
