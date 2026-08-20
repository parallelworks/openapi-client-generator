package analyzer

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	highbase "github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"

	naming "github.com/giraffesyo/openapi-go-naming"
	"github.com/parallelworks/openapi-client-generator/internal/ir"
)

// analyzeOperations walks all paths and operations, populating pkg.Operations.
func (a *Analyzer) analyzeOperations(pkg *ir.Package) error {
	if a.model.Paths == nil || a.model.Paths.PathItems == nil {
		return nil
	}

	for path, pathItem := range a.model.Paths.PathItems.FromOldest() {
		for _, m := range pathOperations(pathItem) {
			opDef, err := a.convertOperation(m.method, path, pathItem, m.op)
			if err != nil {
				return fmt.Errorf("converting %s %s: %w", m.method, path, err)
			}
			pkg.Operations = append(pkg.Operations, opDef)

			defs, warnings := a.callbackPayloads(opDef.Name, m.op)
			pkg.Webhooks = append(pkg.Webhooks, defs...)
			pkg.Warnings = append(pkg.Warnings, warnings...)
		}
	}

	return nil
}

type pathOperation struct {
	method string
	op     *v3high.Operation
}

// pathOperations returns the operations of a path item that get a generated
// method. Everything that reasons about operations ahead of analyzeOperations
// walks this same set, so the two can't disagree about what exists.
func pathOperations(pathItem *v3high.PathItem) []pathOperation {
	all := []pathOperation{
		{"GET", pathItem.Get},
		{"POST", pathItem.Post},
		{"PUT", pathItem.Put},
		{"DELETE", pathItem.Delete},
		{"PATCH", pathItem.Patch},
		{"HEAD", pathItem.Head},
		{"OPTIONS", pathItem.Options},
		{"TRACE", pathItem.Trace},
	}
	return slices.DeleteFunc(all, func(m pathOperation) bool { return m.op == nil })
}

// collectMultipartBodySchemas returns the component schema names that a request
// body sends as multipart form data. Only the content type convertRequestBody
// would pick counts: a schema also offered as JSON is encoded as JSON, so its
// binary properties must stay byte slices.
func (a *Analyzer) collectMultipartBodySchemas() map[string]bool {
	names := make(map[string]bool)
	if a.model.Paths == nil || a.model.Paths.PathItems == nil {
		return names
	}

	for path, pathItem := range a.model.Paths.PathItems.FromOldest() {
		for _, m := range pathOperations(pathItem) {
			if m.op.RequestBody == nil {
				continue
			}
			contentType, mediaType := preferredContent(m.op.RequestBody.Content)
			if !strings.HasPrefix(contentType, "multipart/") || mediaType == nil || mediaType.Schema == nil {
				continue
			}
			if ref := mediaType.Schema.GetReference(); ref != "" {
				a.markMultipartSchema(names, refToSchemaName(ref), 0)
				continue
			}
			// An inline body has no schema name to mark, so it is recorded under
			// the name its synthesized type will be built from. Without this the
			// body is converted as if it were JSON, and its files go out as
			// base64 text in ordinary fields.
			a.inlineMultipartBodies[a.operationName(m.method, path, m.op)+"Body"] = true
		}
	}
	return names
}

// reserveDerivedNames keeps a schema off the identifiers the templates build out
// of an operation or an error body, which share the one package scope with it.
func (a *Analyzer) reserveDerivedNames() {
	for name, pathItem := range a.model.Webhooks.FromOldest() {
		a.reserveInboundNames(naming.Exported(name), "Webhook", pathItem)
	}

	if a.model.Paths == nil || a.model.Paths.PathItems == nil {
		return
	}

	for path, pathItem := range a.model.Paths.PathItems.FromOldest() {
		for _, m := range pathOperations(pathItem) {
			opName := a.operationName(m.method, path, m.op)
			a.namer.Reserve(opName + "Params")
			a.namer.Reserve(opName + "Headers")
			a.reserveCallbackNames(opName, m.op)

			if m.op.Responses == nil || m.op.Responses.Codes == nil {
				continue
			}
			for code, resp := range m.op.Responses.Codes.FromOldest() {
				if isErrorCode(code) {
					a.reserveErrorResponseName(resp)
				}
			}
			a.reserveErrorResponseName(m.op.Responses.Default)
		}
	}
}

// reserveInboundNames reserves the identifiers webhooks.go declares for one
// webhook or callback, so a schema named for one is renamed rather than
// colliding with it.
func (a *Analyzer) reserveInboundNames(goName, suffix string, pathItem *v3high.PathItem) {
	if pathItem == nil {
		return
	}
	ops := pathOperations(pathItem)
	for _, m := range ops {
		name := goName
		if len(ops) > 1 {
			name += naming.Exported(m.method)
		}
		// Only the function the template declares: the payload type is named
		// through the namer like any other, so a schema that wants that name
		// keeps it.
		a.namer.Reserve("Parse" + name + suffix)
	}
}

// reserveCallbackNames reserves what one operation's callbacks declare.
func (a *Analyzer) reserveCallbackNames(opName string, op *v3high.Operation) {
	if op.Callbacks == nil {
		return
	}
	for callbackName, callback := range op.Callbacks.FromOldest() {
		if callback == nil || callback.Expression == nil {
			continue
		}
		for _, pathItem := range callback.Expression.FromOldest() {
			a.reserveInboundNames(opName+naming.Exported(callbackName), "Callback", pathItem)
		}
	}
}

// reserveErrorResponseName reserves the wrapper type errors.go declares for an
// error body.
func (a *Analyzer) reserveErrorResponseName(resp *v3high.Response) {
	if resp == nil || resp.Content == nil {
		return
	}
	for _, mediaType := range resp.Content.FromOldest() {
		if mediaType == nil || mediaType.Schema == nil {
			continue
		}
		if refName := refToSchemaName(mediaType.Schema.GetReference()); refName != "" {
			a.namer.Reserve(naming.Exported(refName) + "Response")
		}
	}
}

// markMultipartSchema marks a schema and everything it composes with allOf, so a
// binary property inherited through composition is still generated as a file.
func (a *Analyzer) markMultipartSchema(names map[string]bool, refName string, depth int) {
	if refName == "" || names[refName] || depth > maxSchemaDepth {
		return
	}
	names[refName] = true

	if a.model.Components == nil || a.model.Components.Schemas == nil {
		return
	}
	proxy, ok := a.model.Components.Schemas.Get(refName)
	if !ok || proxy == nil {
		return
	}
	schema, err := proxy.BuildSchema()
	if err != nil || schema == nil {
		return
	}
	for _, entry := range schema.AllOf {
		a.markMultipartSchema(names, refToSchemaName(entry.GetReference()), depth+1)
	}
}

// maxSchemaDepth bounds a walk over schemas that may refer to one another.
const maxSchemaDepth = 32

// preferredContent picks the media type a request body is sent as: JSON when the
// spec offers a choice, otherwise the first one it lists.
func preferredContent(content *orderedmap.Map[string, *v3high.MediaType]) (string, *v3high.MediaType) {
	if content == nil {
		return "", nil
	}
	var name string
	var chosen *v3high.MediaType
	for contentType, mediaType := range content.FromOldest() {
		isJSON := strings.Contains(contentType, "json")
		if name == "" || isJSON {
			name, chosen = contentType, mediaType
		}
		if isJSON {
			break
		}
	}
	return name, chosen
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
		pd, err := a.convertParam(param, opDef.Name)
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
		rbDef, err := a.convertRequestBody(op.RequestBody, name+"Body")
		if err != nil {
			return nil, fmt.Errorf("converting request body: %w", err)
		}
		opDef.RequestBody = rbDef
	}

	// Responses.
	if op.Responses != nil {
		a.convertResponses(op.Responses, opDef, name)
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
	// Several passes ask for the same operation's name, and every identifier
	// built from it has to agree with the method, so the answer is decided once.
	key := httpMethod + " " + path
	if name, ok := a.opNames[key]; ok {
		return name
	}

	base := naming.Exported(op.OperationId)
	if op.OperationId == "" {
		// Generate from HTTP method + path.
		// e.g., GET /users/{id} → GetUsersByID
		base = naming.Exported(strings.ToLower(httpMethod) + " " + pathToWords(path))
	}

	// Methods live in Client's method set rather than the package scope, so they
	// are numbered against each other and not against a schema that happens to
	// share the name.
	name := base
	for i := 2; a.opNamesTaken[name]; i++ {
		name = base + strconv.Itoa(i)
	}
	a.opNamesTaken[name] = true
	a.opNames[key] = name
	return name
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
func (a *Analyzer) convertParam(param *v3high.Parameter, opName string) (*ir.ParamDef, error) {
	goType := "any"
	contentType := ""
	switch {
	case param.Schema != nil:
		schema, err := param.Schema.BuildSchema()
		if err != nil {
			return nil, fmt.Errorf("building param schema: %w", err)
		}
		if schema != nil {
			goType = a.resolveGoType(schema, opName+naming.Exported(param.Name))
		}
	case param.Content != nil:
		// A parameter with content carries a document, and the media type says how
		// to serialize it. Without reading it the value goes out style-encoded,
		// which is not what the server parses.
		var mediaType *v3high.MediaType
		contentType, mediaType = preferredContent(param.Content)
		if !isJSONContent(contentType) {
			a.warnings = append(a.warnings, fmt.Sprintf("parameter %q: %s is not a media type this generator serializes, so the value is sent style-encoded", param.Name, contentType))
		}
		if mediaType != nil {
			goType = a.resolveMediaTypeSchema(mediaType, opName+naming.Exported(param.Name))
			if goType == "" {
				goType = "any"
			}
		}
	}

	required := param.Required != nil && *param.Required
	style, explode := effectiveStyleExplode(param)

	return &ir.ParamDef{
		Name:          naming.Unexported(param.Name),
		FieldName:     naming.Exported(param.Name),
		OrigName:      param.Name,
		Location:      param.In,
		Type:          goType,
		Required:      required,
		Description:   param.Description,
		Deprecated:    param.Deprecated,
		Style:         style,
		Explode:       explode,
		ContentType:   contentType,
		AllowReserved: param.AllowReserved,
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
func (a *Analyzer) convertRequestBody(rb *v3high.RequestBody, nameHint string) (*ir.RequestBodyDef, error) {
	// The chosen content type decides how the body is encoded on the wire.
	contentType, mediaType := preferredContent(rb.Content)
	if contentType == "" {
		// The spec declares a body but no content to put in it, so there is
		// nothing for the caller to pass and no type to pass it as.
		return nil, nil
	}

	return &ir.RequestBodyDef{
		Required:    rb.Required != nil && *rb.Required,
		Description: rb.Description,
		ContentType: contentType,
		TypeName:    bodyGoType(contentType, a.resolveMediaTypeSchema(mediaType, nameHint), mediaTypeSchema(mediaType)),
	}, nil
}

// formEncodedContentType reports whether a body is sent as form data, whose
// encoders walk the value property by property.
func formEncodedContentType(contentType string) bool {
	return strings.HasPrefix(contentType, "multipart/") ||
		strings.HasPrefix(contentType, "application/x-www-form-urlencoded")
}

// mediaTypeSchema builds a media type's schema, resolving a reference to the
// schema it names.
func mediaTypeSchema(mt *v3high.MediaType) *highbase.Schema {
	if mt == nil || mt.Schema == nil {
		return nil
	}
	schema, err := mt.Schema.BuildSchema()
	if err != nil {
		return nil
	}
	return schema
}

// isObjectLike reports whether a schema describes something with properties to
// walk rather than a scalar or a list.
func isObjectLike(schema *highbase.Schema) bool {
	if schema == nil {
		return false
	}
	if primaryType(schema) == "object" || len(schema.AllOf) > 0 {
		return true
	}
	return schema.Properties != nil && schema.Properties.Len() > 0
}

// bodyGoType is the Go type a request body is accepted as. A body the client
// cannot structurally encode — XML, say — is taken as the bytes or text it
// already is rather than as a struct there would be no encoder for, and a body
// the spec declares without a schema still needs some type to be passed as.
func bodyGoType(contentType, typeName string, schema *highbase.Schema) string {
	switch {
	case strings.Contains(contentType, "json"):
		if typeName == "" {
			return "any"
		}
		return typeName
	case formEncodedContentType(contentType):
		// The form encoders build parts and pairs out of an object's properties,
		// so a body that is not an object gives them nothing to work from.
		if isObjectLike(schema) {
			return typeName
		}
		return "map[string]any"
	}
	switch typeName {
	case "string", "[]byte":
		return typeName
	}
	if strings.HasPrefix(contentType, "text/") {
		return "string"
	}
	return "[]byte"
}

// convertResponses converts operation responses into the OperationDef fields.
func (a *Analyzer) convertResponses(responses *v3high.Responses, opDef *ir.OperationDef, opName string) {
	if responses.Codes != nil {
		for code, resp := range responses.Codes.FromOldest() {
			// Only the success body reaches the method signature, so it keeps the
			// plain <Op>Response hint; the others carry their status code so two
			// inline bodies of one operation can't land on the same name.
			hint := opName + "Response" + code
			if isSuccessCode(code) && opDef.SuccessResponse == nil {
				hint = opName + "Response"
			}
			rd := a.convertSingleResponse(code, resp, hint)
			opDef.Responses = append(opDef.Responses, rd)

			if isSuccessCode(code) {
				if opDef.SuccessResponse == nil {
					opDef.SuccessResponse = rd
				}
			} else if isErrorCode(code) {
				rd.IsError = true
				rd.ErrorWrapper = a.errorWrapperName(rd.TypeName, hint)
				opDef.ErrorResponses = append(opDef.ErrorResponses, rd)
			}
		}
	}

	// Handle the default response.
	if responses.Default != nil {
		hint := opName + "DefaultResponse"
		rd := a.convertSingleResponse("default", responses.Default, hint)
		rd.IsError = true
		rd.ErrorWrapper = a.errorWrapperName(rd.TypeName, hint)
		opDef.Responses = append(opDef.Responses, rd)
		opDef.ErrorResponses = append(opDef.ErrorResponses, rd)
	}
}

// errorWrapperName returns the type name for the wrapper that carries an error
// body parsed into Detail. A body whose Go type is a map, a slice, or a builtin
// has no name an identifier can be built from, so the wrapper takes the
// operation's instead of pasting the type expression into the declaration.
func (a *Analyzer) errorWrapperName(typeName, hint string) string {
	if typeName == "" {
		return ""
	}
	named := ir.NamedType(typeName)
	if r, _ := utf8.DecodeRuneInString(named); unicode.IsUpper(r) {
		return named + "Response"
	}
	return a.namer.Unique(naming.Exported(hint) + "Error")
}

// convertSingleResponse converts one response code/definition to an ir.ResponseDef.
func (a *Analyzer) convertSingleResponse(code string, resp *v3high.Response, nameHint string) *ir.ResponseDef {
	rd := &ir.ResponseDef{
		StatusCode:  code,
		Description: resp.Description,
	}

	rd.Headers = convertResponseHeaders(resp)

	if resp.Content != nil && resp.Content.Len() > 1 {
		a.multiContentResponses++
	}

	if resp.Links != nil && resp.Links.Len() > 0 {
		a.linksSeen = true
	}

	if resp.Content != nil {
		for contentType, mediaType := range resp.Content.FromOldest() {
			if strings.Contains(contentType, "json") {
				rd.ContentType = contentType
				rd.TypeName = a.resolveMediaTypeSchema(mediaType, nameHint)
				break
			}
		}
		// If no JSON, take the first.
		if rd.ContentType == "" {
			for contentType, mediaType := range resp.Content.FromOldest() {
				rd.ContentType = contentType
				rd.TypeName = a.resolveMediaTypeSchema(mediaType, nameHint)
				if rd.TypeName == "" && strings.HasPrefix(contentType, "text/") {
					rd.TypeName = "string"
				}
				break
			}
		}
	}

	return rd
}

// isJSONContent reports whether a media type is one the generated code can
// serialize a parameter value into.
func isJSONContent(contentType string) bool {
	return strings.Contains(contentType, "json")
}

// convertResponseHeaders lowers the headers a response declares. A header value
// arrives as text, so only the kinds text parses into unambiguously are typed;
// everything else, dates and lists included, stays the raw string.
func convertResponseHeaders(resp *v3high.Response) []*ir.ResponseHeaderDef {
	if resp.Headers == nil {
		return nil
	}
	var headers []*ir.ResponseHeaderDef
	for name, header := range resp.Headers.FromOldest() {
		if header == nil {
			continue
		}
		hd := &ir.ResponseHeaderDef{
			Name:        name,
			GoName:      naming.Exported(name),
			Type:        "string",
			Description: header.Description,
			Required:    header.Required,
		}
		if header.Schema != nil {
			if schema, err := header.Schema.BuildSchema(); err == nil && schema != nil {
				hd.Type = headerGoType(schema)
			}
		}
		headers = append(headers, hd)
	}
	return headers
}

// headerGoType maps a header's schema to the Go type its value parses into.
func headerGoType(schema *highbase.Schema) string {
	switch primaryType(schema) {
	case "integer":
		return "int64"
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	}
	return "string"
}

// resolveMediaTypeSchema extracts the Go type name from a media type's schema.
func (a *Analyzer) resolveMediaTypeSchema(mt *v3high.MediaType, nameHint string) string {
	if mt == nil || mt.Schema == nil {
		return ""
	}

	// Check for a $ref first.
	if goType := a.goTypeForRef(mt.Schema.GetReference()); goType != "" {
		return goType
	}

	schema, err := mt.Schema.BuildSchema()
	if err != nil || schema == nil {
		return ""
	}
	return a.resolveGoType(schema, nameHint)
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
