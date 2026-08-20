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
