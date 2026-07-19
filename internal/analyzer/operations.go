package analyzer

import (
	"fmt"
	"strings"

	highbase "github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	naming "github.com/giraffesyo/openapi-go-naming"
	"github.com/parallelworks/openapi-client-generator/internal/ir"
)

// analyzeOperations walks all paths and operations, populating pkg.Operations.
func (a *Analyzer) analyzeOperations(pkg *ir.Package) error {
	if a.model.Paths == nil || a.model.Paths.PathItems == nil {
		return nil
	}

	for path, pathItem := range a.model.Paths.PathItems.FromOldest() {
		methods := []struct {
			method string
			op     *v3high.Operation
		}{
			{"GET", pathItem.Get},
			{"POST", pathItem.Post},
			{"PUT", pathItem.Put},
			{"DELETE", pathItem.Delete},
			{"PATCH", pathItem.Patch},
			{"HEAD", pathItem.Head},
			{"OPTIONS", pathItem.Options},
		}

		for _, m := range methods {
			if m.op == nil {
				continue
			}

			opDef, err := a.convertOperation(m.method, path, pathItem, m.op)
			if err != nil {
				return fmt.Errorf("converting %s %s: %w", m.method, path, err)
			}
			pkg.Operations = append(pkg.Operations, opDef)
		}
	}

	return nil
}

// convertOperation converts a single OpenAPI operation into an ir.OperationDef.
func (a *Analyzer) convertOperation(httpMethod, path string, pathItem *v3high.PathItem, op *v3high.Operation) (*ir.OperationDef, error) {
	name := a.operationName(httpMethod, path, op)

	opDef := &ir.OperationDef{
		Name:        name,
		Summary:     op.Summary,
		Description: op.Description,
		HTTPMethod:  httpMethod,
		Path:        path,
		Tags:        op.Tags,
		Deprecated:  op.Deprecated != nil && *op.Deprecated,
	}

	// Merge path-level and operation-level parameters.
	// Operation-level params override path-level ones with the same name+location.
	params := mergeParams(pathItem.Parameters, op.Parameters)
	for _, param := range params {
		pd, err := a.convertParam(param)
		if err != nil {
			return nil, fmt.Errorf("converting parameter %q: %w", param.Name, err)
		}
		switch pd.Location {
		case "path":
			opDef.PathParams = append(opDef.PathParams, pd)
		case "query":
			opDef.QueryParams = append(opDef.QueryParams, pd)
		case "header":
			opDef.HeaderParams = append(opDef.HeaderParams, pd)
		case "cookie":
			opDef.CookieParams = append(opDef.CookieParams, pd)
		}
	}

	// Request body.
	if op.RequestBody != nil {
		rbDef, err := a.convertRequestBody(op.RequestBody)
		if err != nil {
			return nil, fmt.Errorf("converting request body: %w", err)
		}
		opDef.RequestBody = rbDef
	}

	// Responses.
	if op.Responses != nil {
		a.convertResponses(op.Responses, opDef)
	}

	// Security requirements.
	if len(op.Security) > 0 {
		opDef.SecurityReqs = convertSecurityReqs(op.Security)
	}

	disambiguateParamNames(opDef)

	return opDef, nil
}

// disambiguateParamNames renames generated identifiers that would otherwise
// collide, suffixing by kind; only Go identifiers change, never the wire OrigName.
func disambiguateParamNames(opDef *ir.OperationDef) {
	// Reserved: the receiver, args, and method/iterator locals a path param could
	// shadow, the package identifiers the generated body references (e.g. a param
	// named `url` would shadow the net/url import in `url.Values{}`), and the
	// helper functions the method body calls (a param named `add_query_param`
	// becomes `addQueryParam`, shadowing the helper of that name).
	posUsed := map[string]bool{
		"c": true, "ctx": true, "path": true, "queryValues": true,
		"headers": true, "result": true, "err": true,
		"cursor": true, "p": true, "next": true,
		"context": true, "fmt": true, "http": true, "url": true,
		"pathReplace": true, "addQueryParam": true, "encodeQuery": true,
		"setHeader": true, "addCookieHeader": true,
	}
	if opDef.RequestBody != nil {
		posUsed["body"] = true
	}
	if len(opDef.QueryParams) > 0 || len(opDef.HeaderParams) > 0 || len(opDef.CookieParams) > 0 {
		posUsed["params"] = true
		posUsed["opts"] = true
	}
	for _, p := range opDef.PathParams {
		name := p.Name
		for posUsed[name] {
			name += "Path"
		}
		posUsed[name] = true
		p.Name = name
	}

	// Query, header, and cookie params share one struct, so dedupe field names by location.
	fieldUsed := map[string]bool{}
	dedupeField := func(p *ir.ParamDef, suffix string) {
		name := p.FieldName
		for fieldUsed[name] {
			name += suffix
		}
		fieldUsed[name] = true
		p.FieldName = name
	}
	for _, p := range opDef.QueryParams {
		dedupeField(p, "Query")
	}
	for _, p := range opDef.HeaderParams {
		dedupeField(p, "Header")
	}
	for _, p := range opDef.CookieParams {
		dedupeField(p, "Cookie")
	}
}

// operationName determines the Go method name for an operation.
func (a *Analyzer) operationName(httpMethod, path string, op *v3high.Operation) string {
	if op.OperationId != "" {
		return naming.Exported(op.OperationId)
	}
	// Generate from HTTP method + path.
	// e.g., GET /users/{id} → GetUsersByID
	return naming.Exported(strings.ToLower(httpMethod) + " " + pathToWords(path))
}

// pathToWords converts a URL path to space-separated words for naming.
// e.g., "/users/{userId}/posts" → "users userId posts"
func pathToWords(path string) string {
	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "{", "")
	path = strings.ReplaceAll(path, "}", "")
	path = strings.ReplaceAll(path, "/", " ")
	return path
}

// mergeParams merges path-level and operation-level parameters.
// Operation-level parameters override path-level ones with the same name and location.
func mergeParams(pathParams, opParams []*v3high.Parameter) []*v3high.Parameter {
	if len(pathParams) == 0 {
		return opParams
	}
	if len(opParams) == 0 {
		return pathParams
	}

	// Build a set of operation-level param keys (name+in).
	opKeys := make(map[string]bool, len(opParams))
	for _, p := range opParams {
		opKeys[p.Name+"|"+p.In] = true
	}

	// Start with path-level params not overridden by operation-level.
	var merged []*v3high.Parameter
	for _, p := range pathParams {
		if !opKeys[p.Name+"|"+p.In] {
			merged = append(merged, p)
		}
	}
	// Then add all operation-level params.
	merged = append(merged, opParams...)
	return merged
}

// convertParam converts an OpenAPI parameter to an ir.ParamDef.
func (a *Analyzer) convertParam(param *v3high.Parameter) (*ir.ParamDef, error) {
	goType := "any"
	if param.Schema != nil {
		schema, err := param.Schema.BuildSchema()
		if err != nil {
			return nil, fmt.Errorf("building param schema: %w", err)
		}
		if schema != nil {
			goType = a.resolveGoType(schema, "")
		}
	}

	required := param.Required != nil && *param.Required
	style, explode := effectiveStyleExplode(param)

	return &ir.ParamDef{
		Name:        naming.Unexported(param.Name),
		FieldName:   naming.Exported(param.Name),
		OrigName:    param.Name,
		Location:    param.In,
		Type:        goType,
		Required:    required,
		Description: param.Description,
		Deprecated:  param.Deprecated,
		Style:       style,
		Explode:     explode,
	}, nil
}

// effectiveStyleExplode resolves the OpenAPI serialization defaults: style is
// form for query/cookie and simple for path/header when unset; explode defaults
// to true only for form. The raw param.Explode is false when omitted, which would
// wrongly collapse an ordinary form array — so the default must be applied here.
func effectiveStyleExplode(param *v3high.Parameter) (string, bool) {
	style := param.Style
	if style == "" {
		switch param.In {
		case "query", "cookie":
			style = "form"
		default:
			style = "simple"
		}
	}
	explode := style == "form"
	if param.Explode != nil {
		explode = *param.Explode
	}
	return style, explode
}

// convertRequestBody converts an OpenAPI request body to an ir.RequestBodyDef.
func (a *Analyzer) convertRequestBody(rb *v3high.RequestBody) (*ir.RequestBodyDef, error) {
	def := &ir.RequestBodyDef{
		Required:    rb.Required != nil && *rb.Required,
		Description: rb.Description,
	}

	if rb.Content == nil {
		return def, nil
	}

	// Prefer application/json content type.
	for contentType, mediaType := range rb.Content.FromOldest() {
		if strings.Contains(contentType, "json") {
			def.ContentType = contentType
			def.TypeName = a.resolveMediaTypeSchema(mediaType)
			break
		}
		if strings.Contains(contentType, "multipart") {
			def.ContentType = contentType
			def.IsMultipart = true
			def.TypeName = a.resolveMediaTypeSchema(mediaType)
			break
		}
	}

	// If no JSON or multipart found, take the first content type.
	if def.ContentType == "" {
		for contentType, mediaType := range rb.Content.FromOldest() {
			def.ContentType = contentType
			def.TypeName = a.resolveMediaTypeSchema(mediaType)
			break
		}
	}

	return def, nil
}

// convertResponses converts operation responses into the OperationDef fields.
func (a *Analyzer) convertResponses(responses *v3high.Responses, opDef *ir.OperationDef) {
	if responses.Codes != nil {
		for code, resp := range responses.Codes.FromOldest() {
			rd := a.convertSingleResponse(code, resp)
			opDef.Responses = append(opDef.Responses, rd)

			if isSuccessCode(code) {
				if opDef.SuccessResponse == nil {
					opDef.SuccessResponse = rd
				}
			} else if isErrorCode(code) {
				rd.IsError = true
				opDef.ErrorResponses = append(opDef.ErrorResponses, rd)
			}
		}
	}

	// Handle the default response.
	if responses.Default != nil {
		rd := a.convertSingleResponse("default", responses.Default)
		rd.IsError = true
		opDef.Responses = append(opDef.Responses, rd)
		opDef.ErrorResponses = append(opDef.ErrorResponses, rd)
	}
}

// convertSingleResponse converts one response code/definition to an ir.ResponseDef.
func (a *Analyzer) convertSingleResponse(code string, resp *v3high.Response) *ir.ResponseDef {
	rd := &ir.ResponseDef{
		StatusCode:  code,
		Description: resp.Description,
	}

	if resp.Content != nil {
		for contentType, mediaType := range resp.Content.FromOldest() {
			if strings.Contains(contentType, "json") {
				rd.ContentType = contentType
				rd.TypeName = a.resolveMediaTypeSchema(mediaType)
				break
			}
		}
		// If no JSON, take the first.
		if rd.ContentType == "" {
			for contentType, mediaType := range resp.Content.FromOldest() {
				rd.ContentType = contentType
				rd.TypeName = a.resolveMediaTypeSchema(mediaType)
				if rd.TypeName == "" && strings.HasPrefix(contentType, "text/") {
					rd.TypeName = "string"
				}
				break
			}
		}
	}

	return rd
}

// resolveMediaTypeSchema extracts the Go type name from a media type's schema.
func (a *Analyzer) resolveMediaTypeSchema(mt *v3high.MediaType) string {
	if mt == nil || mt.Schema == nil {
		return ""
	}

	// Check for a $ref first.
	ref := mt.Schema.GetReference()
	if ref != "" {
		refName := refToSchemaName(ref)
		if refName != "" {
			if td, ok := a.typesBySchema[refName]; ok {
				return td.Name
			}
			return naming.Exported(refName)
		}
	}

	schema, err := mt.Schema.BuildSchema()
	if err != nil || schema == nil {
		return ""
	}
	return a.resolveGoType(schema, "")
}

// convertSecurityReqs converts OpenAPI security requirements to IR.
func convertSecurityReqs(reqs []*highbase.SecurityRequirement) [][]ir.SecurityReq {
	var result [][]ir.SecurityReq
	for _, req := range reqs {
		if req == nil || req.Requirements == nil {
			continue
		}
		var andGroup []ir.SecurityReq
		for schemeName, scopes := range req.Requirements.FromOldest() {
			andGroup = append(andGroup, ir.SecurityReq{
				SchemeName: schemeName,
				Scopes:     scopes,
			})
		}
		if len(andGroup) > 0 {
			result = append(result, andGroup)
		}
	}
	return result
}

// isSuccessCode returns true if the HTTP status code string is a 2xx code.
func isSuccessCode(code string) bool {
	return len(code) == 3 && code[0] == '2'
}

// isErrorCode returns true if the HTTP status code string is 4xx or 5xx.
func isErrorCode(code string) bool {
	return len(code) == 3 && (code[0] == '4' || code[0] == '5')
}
