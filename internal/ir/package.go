package ir

// Package is the top-level IR representing the entire generated package.
type Package struct {
	Name        string           // Go package name
	Types       []*TypeDef       // All type definitions
	Operations  []*OperationDef  // All API operations
	AuthSchemes []*AuthScheme    // Security schemes from the spec
	ServerURLs  []string         // Default server URLs
	Info        *APIInfo         // API title, version, description
}

// APIInfo contains metadata about the API.
type APIInfo struct {
	Title       string
	Version     string
	Description string
}
