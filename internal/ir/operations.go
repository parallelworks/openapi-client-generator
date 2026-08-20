package ir

// OperationDef represents a single API operation.
type OperationDef struct {
	Name            string // Go method name (e.g., "ListUsers")
	Summary         string // Short summary from the spec
	Description     string
	HTTPMethod      string // "GET", "POST", etc.
	Path            string // URL path template (e.g., "/users/{id}")
	Tags            []string
	PathParams      []*ParamDef
	QueryParams     []*ParamDef
	HeaderParams    []*ParamDef
	CookieParams    []*ParamDef
	RequestBody     *RequestBodyDef // nil if no body
	Responses       []*ResponseDef
	SuccessResponse *ResponseDef    // The primary 2xx response
	ErrorResponses  []*ResponseDef  // 4xx/5xx responses
	SecurityReqs    [][]SecurityReq // OR of (AND of scheme refs)
	Deprecated      bool
	Pagination      *PaginationDef // nil if not paginated
}

// ParamDef represents an operation parameter.
type ParamDef struct {
	Name        string // Go parameter name (camelCase)
	FieldName   string // Go struct field name (PascalCase) for params struct
	OrigName    string // Original parameter name from spec
	Location    string // "path", "query", "header", "cookie"
	Type        string // Go type expression
	Required    bool
	Description string
	Deprecated  bool
	Style       string // serialization style
	Explode     bool
	ContentType string // Media type when the parameter is serialized with content rather than a style
}

// RequestBodyDef describes the request body.
type RequestBodyDef struct {
	Required    bool
	Description string
	ContentType string // Primary content type (e.g., "application/json")
	TypeName    string // Go type for the body
}

// ResponseDef describes one response.
type ResponseDef struct {
	StatusCode   string // "200", "404", "default", etc.
	Description  string
	ContentType  string
	TypeName     string // Go type for the response body (empty if no body)
	ErrorWrapper string // Go type name of the generated wrapper carrying the parsed body
	IsError      bool   // Whether this is an error response (4xx/5xx)
	Headers      []*ResponseHeaderDef
}

// ResponseHeaderDef describes a response header.
type ResponseHeaderDef struct {
	Name        string
	GoName      string
	Type        string // string, int64, float64, or bool
	Description string
	Required    bool
}

// SecurityReq represents a single security requirement.
type SecurityReq struct {
	SchemeName string
	Scopes     []string
}
