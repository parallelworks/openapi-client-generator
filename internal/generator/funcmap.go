package generator

import (
	"slices"
	"strconv"
	"strings"
	"sync"
	"text/template"

	naming "github.com/giraffesyo/openapi-go-naming"
	"github.com/parallelworks/openapi-client-generator/internal/ir"
)

// FuncMap returns the template.FuncMap used by all templates.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"cleanDoc":                cleanDoc,
		"opDocComment":            opDocComment,
		"typeDocComment":          typeDocComment,
		"fieldDocComment":         fieldDocComment,
		"paramDocComment":         paramDocComment,
		"indent":                  indent,
		"jsonTag":                 jsonTag,
		"fieldTag":                fieldTag,
		"hasOperations":           hasOperations,
		"successType":             successType,
		"hasBody":                 hasBody,
		"hasRequiredQueryParams":  hasRequiredQueryParams,
		"hasRequiredHeaderParams": hasRequiredHeaderParams,
		"hasRequiredCookieParams": hasRequiredCookieParams,
		"paramType":               paramType,
		"hasUnions":               hasUnions,
		"hasUntypedVariant":       hasUntypedVariant,
		"distinctVariants":        distinctVariants,
		"unionBaseFields":         unionBaseFields,
		"catchAllField":           catchAllField,
		"catchAllValueType":       catchAllValueType,
		"hasCatchAllTypes":        hasCatchAllTypes,
		"marshalerEmbeds":         marshalerEmbeds,
		"plainFields":             plainFields,
		"fieldGoName":             fieldGoName,
		"embeddedCatchAlls":       embeddedCatchAlls,
		"hasMarshalerEmbeds":      hasMarshalerEmbeds,
		"decodedEmbeds":           decodedEmbeds,
		"opaqueEmbeds":            opaqueEmbeds,
		"declaredNamesLiteral":    declaredNamesLiteral,
		"discriminatorFieldName":  discriminatorFieldName,
		"hasPaginatedOps":         hasPaginatedOps,
		"paginationItemType":      paginationItemType,
		"paginationCursorField":   paginationCursorField,
		"toGoName":                naming.Exported,
		"uniqueErrorTypes":        uniqueErrorTypes,
		"errorMessageField":       errorMessageField,
		"errorType":               errorType,
		"successContentType":      successContentType,
		"requestContentType":      requestContentType,
		"hasNonJSONBody":          hasNonJSONBody,
		"primaryServer":           primaryServer,
		"defaultBaseURL":          defaultBaseURL,
		"operationHeaders":        operationHeaders,
		"inboundKinds":            inboundKinds,
		"inboundPayloads":         inboundPayloads,
		"serializedParam":         serializedParam,
		"reservedQueryParams":     reservedQueryParams,
		"headerKinds":             headerKinds,
		"headerDocComment":        headerDocComment,
	}
}

// cleanDoc cleans a description string for use in GoDoc comments.
// It replaces newlines with spaces and trims whitespace.
func cleanDoc(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// commentLines splits a description into properly prefixed Go comment lines.
// Each line gets a "// " prefix; blank lines produce "//".
func commentLines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			lines = append(lines, "//")
		} else {
			lines = append(lines, "// "+line)
		}
	}
	return lines
}

// typeDocComment generates a Go doc comment for a type definition.
// Format: "// TypeName - description" with multi-line support.
func typeDocComment(td *ir.TypeDef) string {
	desc := strings.TrimSpace(td.Description)
	if desc == "" {
		return ""
	}
	desc = strings.ReplaceAll(desc, "\r\n", "\n")
	lines := strings.Split(desc, "\n")
	var parts []string
	parts = append(parts, "// "+td.Name+" - "+strings.TrimRight(lines[0], " \t"))
	for _, line := range lines[1:] {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			parts = append(parts, "//")
		} else {
			parts = append(parts, "// "+line)
		}
	}
	return strings.Join(parts, "\n")
}

// opDocComment generates a complete Go doc comment for an operation method.
// It combines Summary (first line), Description (body), and Deprecated marker.
func opDocComment(op *ir.OperationDef) string {
	summary := strings.TrimSpace(op.Summary)
	desc := strings.TrimSpace(op.Description)

	var parts []string

	if summary != "" && desc != "" {
		parts = append(parts, "// "+op.Name+" - "+summary)
		parts = append(parts, "//")
		parts = append(parts, commentLines(desc)...)
	} else if summary != "" {
		parts = append(parts, "// "+op.Name+" - "+summary)
	} else if desc != "" {
		descLines := commentLines(desc)
		// Prefix the method name on the first line.
		if len(descLines) > 0 {
			descLines[0] = "// " + op.Name + " - " + strings.TrimPrefix(descLines[0], "// ")
		}
		parts = append(parts, descLines...)
	}

	if op.Deprecated {
		if len(parts) > 0 {
			parts = append(parts, "//")
		}
		parts = append(parts, "// Deprecated: this operation is deprecated.")
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

// fieldDocComment generates a Go doc comment for a struct field.
// Includes description and a Deprecated marker if applicable.
func fieldDocComment(f *ir.Field) string {
	var parts []string
	parts = append(parts, commentLines(f.Description)...)
	if f.Deprecated {
		if len(parts) > 0 {
			parts = append(parts, "//")
		}
		parts = append(parts, "// Deprecated: this field is deprecated.")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

// paramDocComment generates a Go doc comment for a parameter struct field.
// Includes description and a Deprecated marker if applicable.
func paramDocComment(p *ir.ParamDef) string {
	var parts []string
	parts = append(parts, commentLines(p.Description)...)
	if p.Deprecated {
		if len(parts) > 0 {
			parts = append(parts, "//")
		}
		parts = append(parts, "// Deprecated: this parameter is deprecated.")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

// indent prepends a tab character to each non-empty line of s.
func indent(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = "\t" + line
		}
	}
	return strings.Join(lines, "\n")
}

// fieldTag returns a struct field's full tag. The catch-all carries a marker
// because its json tag is "-": the body encoders have no other way to tell it
// apart from a field the schema genuinely excludes.
func fieldTag(f *ir.Field) string {
	tag := `json:"` + jsonTag(f) + `"`
	if f.CatchAll {
		tag += ` openapi:"additionalProperties"`
	}
	return tag
}

// jsonTag returns the JSON struct tag value for a field.
// It returns "fieldName,omitempty" for optional fields and "fieldName" for required ones.
func jsonTag(f *ir.Field) string {
	// encoding/json only skips a field when the tag is exactly "-"; appending
	// anything turns it into a property literally named "-".
	if f.JSONName == "-" {
		return "-"
	}
	tag := f.JSONName
	if f.OmitEmpty {
		tag += ",omitempty"
	}
	return tag
}

func catchAllField(td *ir.TypeDef) *ir.Field {
	if td.Kind != ir.TypeKindStruct {
		return nil
	}
	for _, f := range td.Fields {
		if f.CatchAll {
			return f
		}
	}
	return nil
}

func catchAllValueType(f *ir.Field) string {
	return strings.TrimPrefix(f.Type, "map[string]")
}

// declaredJSONNames returns the wire names a struct already consumes into
// fields, an embedded type's included: those are promoted onto the struct, so a
// catch-all that re-collected them would emit each one twice.
func declaredJSONNames(pkg *ir.Package, td *ir.TypeDef) []string {
	byName := typeIndex(pkg).byName

	var names []string
	visited := make(map[string]bool)

	var walk func(td *ir.TypeDef)
	walk = func(td *ir.TypeDef) {
		if td == nil || visited[td.Name] {
			return
		}
		visited[td.Name] = true
		for _, f := range td.Fields {
			switch {
			case f.CatchAll:
			case f.Embedded:
				// The type may be written as a pointer where an indirection broke a
				// reference cycle, and may be an alias standing for the struct.
				walk(ir.StructNamed(byName, strings.TrimPrefix(f.Type, "*")))
			case f.JSONName == "" || f.JSONName == "-":
			default:
				names = append(names, f.JSONName)
			}
		}
	}
	walk(td)
	return names
}

// declaredNamesLiteral renders declaredJSONNames as a Go []string literal.
func declaredNamesLiteral(pkg *ir.Package, td *ir.TypeDef) string {
	names := declaredJSONNames(pkg, td)
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = strconv.Quote(name)
	}
	return "[]string{" + strings.Join(quoted, ", ") + "}"
}

func hasCatchAllTypes(types []*ir.TypeDef) bool {
	return slices.ContainsFunc(types, func(td *ir.TypeDef) bool {
		return catchAllField(td) != nil
	})
}

// terminalTypeName resolves a Go type expression to the named type it denotes,
// following aliases, or "" for builtins and composites.
func terminalTypeName(byName map[string]*ir.TypeDef, goType string) string {
	name := ir.NamedType(strings.TrimPrefix(goType, "*"))
	for range len(byName) + 1 {
		td := byName[name]
		if td == nil || td.Kind != ir.TypeKindAlias {
			return name
		}
		next := ir.NamedType(strings.TrimPrefix(td.GoType, "*"))
		if next == "" {
			return ""
		}
		name = next
	}
	return name
}

// pkgTypeIndex carries the package-wide lookups the marshaler helpers share.
// bearing holds the names of generated types whose method set carries
// MarshalJSON/UnmarshalJSON: unions, structs with a catch-all, and structs that
// embed such a type and so inherit the methods by promotion.
type pkgTypeIndex struct {
	byName  map[string]*ir.TypeDef
	bearing map[string]bool
}

// The bearing fixed point is package-wide but templates ask per type, so the
// last package's index is cached. A single entry never outlives its package by
// more than one generate and stays safe under concurrent generates.
var typeIndexMu sync.Mutex
var lastTypeIndex struct {
	pkg *ir.Package
	idx *pkgTypeIndex
}

func typeIndex(pkg *ir.Package) *pkgTypeIndex {
	typeIndexMu.Lock()
	defer typeIndexMu.Unlock()
	if lastTypeIndex.pkg == pkg {
		return lastTypeIndex.idx
	}
	idx := &pkgTypeIndex{byName: ir.TypesByName(pkg.Types), bearing: map[string]bool{}}
	for _, td := range pkg.Types {
		if td == nil {
			continue
		}
		if td.Kind == ir.TypeKindUnion || catchAllField(td) != nil {
			idx.bearing[td.Name] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, td := range pkg.Types {
			if td == nil || td.Kind != ir.TypeKindStruct || idx.bearing[td.Name] {
				continue
			}
			for _, f := range td.Fields {
				if f.Embedded && idx.bearing[terminalTypeName(idx.byName, f.Type)] {
					idx.bearing[td.Name] = true
					changed = true
					break
				}
			}
		}
	}
	lastTypeIndex.pkg, lastTypeIndex.idx = pkg, idx
	return idx
}

// marshalerEmbeds returns the embedded fields whose types carry custom JSON
// marshalers, which the enclosing struct must encode part by part: handing the
// whole struct to encoding/json would promote those methods and let one
// embedded type speak for the entire object. A struct whose single field is
// such an embed has nothing of its own to lose to promotion, so it gets none.
func marshalerEmbeds(pkg *ir.Package, td *ir.TypeDef) []*ir.Field {
	if td.Kind != ir.TypeKindStruct || len(td.Fields) < 2 {
		return nil
	}
	idx := typeIndex(pkg)
	var embeds []*ir.Field
	for _, f := range td.Fields {
		if f.Embedded && idx.bearing[terminalTypeName(idx.byName, f.Type)] {
			embeds = append(embeds, f)
		}
	}
	return embeds
}

// decodedEmbeds returns the marshaler embeds UnmarshalJSON decodes in place. A
// pointer embed that closes a reference cycle stays nil: decoding it would
// re-enter this unmarshaler with the same bytes and recurse until the stack
// overflows, and encoding/json also stops its field walk where a type repeats.
func decodedEmbeds(pkg *ir.Package, td *ir.TypeDef) []*ir.Field {
	byName := typeIndex(pkg).byName
	var embeds []*ir.Field
	for _, f := range marshalerEmbeds(pkg, td) {
		if strings.HasPrefix(f.Type, "*") && embedsReach(byName, f.Type, td.Name) {
			continue
		}
		embeds = append(embeds, f)
	}
	return embeds
}

func embedsReach(byName map[string]*ir.TypeDef, goType, target string) bool {
	seen := map[string]bool{}
	var walk func(goType string) bool
	walk = func(goType string) bool {
		td := ir.StructNamed(byName, strings.TrimPrefix(goType, "*"))
		if td == nil || seen[td.Name] {
			return false
		}
		if td.Name == target {
			return true
		}
		seen[td.Name] = true
		for _, f := range td.Fields {
			if f.Embedded && walk(f.Type) {
				return true
			}
		}
		return false
	}
	return walk(goType)
}

// opaqueEmbeds returns the marshaler embeds whose wire keys cannot be listed
// statically (unions): the unmarshaler prunes the catch-all with the keys the
// decoded value actually marshals instead.
func opaqueEmbeds(pkg *ir.Package, td *ir.TypeDef) []*ir.Field {
	byName := typeIndex(pkg).byName
	var embeds []*ir.Field
	for _, f := range marshalerEmbeds(pkg, td) {
		if ir.StructNamed(byName, strings.TrimPrefix(f.Type, "*")) == nil {
			embeds = append(embeds, f)
		}
	}
	return embeds
}

// plainFields returns the fields a part-wise marshaled struct encodes directly:
// everything except the catch-all and the marshaler-bearing embeds.
func plainFields(pkg *ir.Package, td *ir.TypeDef) []*ir.Field {
	skip := make(map[*ir.Field]bool)
	for _, f := range marshalerEmbeds(pkg, td) {
		skip[f] = true
	}
	var fields []*ir.Field
	for _, f := range td.Fields {
		if !f.CatchAll && !skip[f] {
			fields = append(fields, f)
		}
	}
	return fields
}

// fieldGoName returns the name a field is selected by: for an embedded field
// that is the type name, with any pointer indirection stripped.
func fieldGoName(f *ir.Field) string {
	if f.Embedded {
		return strings.TrimPrefix(f.Type, "*")
	}
	return f.Name
}

// EmbeddedCatchAll locates a catch-all map inside an embedded type: the Go
// selector path from the enclosing struct, and the nil checks any pointers on
// that path require.
type EmbeddedCatchAll struct {
	Guard string
	Path  string
}

// embeddedCatchAlls returns the catch-all maps reachable through a struct's
// embedded types, which the enclosing type's UnmarshalJSON has to clean up
// after the embedded unmarshalers have run.
func embeddedCatchAlls(pkg *ir.Package, td *ir.TypeDef) []EmbeddedCatchAll {
	byName := typeIndex(pkg).byName

	var found []EmbeddedCatchAll
	seen := map[string]bool{td.Name: true}
	var walk func(td *ir.TypeDef, path, guard string)
	walk = func(td *ir.TypeDef, path, guard string) {
		for _, f := range td.Fields {
			if !f.Embedded {
				continue
			}
			embedded := ir.StructNamed(byName, fieldGoName(f))
			if embedded == nil || seen[embedded.Name] {
				continue
			}
			fieldPath := path + fieldGoName(f)
			fieldGuard := guard
			if strings.HasPrefix(f.Type, "*") {
				if fieldGuard != "" {
					fieldGuard += " && "
				}
				fieldGuard += "t." + fieldPath + " != nil"
			}
			if ca := catchAllField(embedded); ca != nil {
				found = append(found, EmbeddedCatchAll{Guard: fieldGuard, Path: fieldPath + "." + ca.Name})
			}
			seen[embedded.Name] = true
			walk(embedded, fieldPath+".", fieldGuard)
			delete(seen, embedded.Name)
		}
	}
	walk(td, "", "")
	return found
}

// hasMarshalerEmbeds reports whether any struct needs part-wise marshalers,
// which is what pulls the JSON object merge helpers into the generated types.
func hasMarshalerEmbeds(pkg *ir.Package) bool {
	return slices.ContainsFunc(pkg.Types, func(td *ir.TypeDef) bool {
		return td != nil && len(marshalerEmbeds(pkg, td)) > 0
	})
}

// hasOperations returns true if the package has any operations defined.
func hasOperations(pkg *ir.Package) bool {
	return len(pkg.Operations) > 0
}

// successType returns the Go type name for an operation's success response.
// Returns empty string if there is no success response body.
func successType(op *ir.OperationDef) string {
	if op.SuccessResponse == nil {
		return ""
	}
	return op.SuccessResponse.TypeName
}

// hasBody returns true if the operation has a request body.
func hasBody(op *ir.OperationDef) bool {
	return op.RequestBody != nil
}

// hasRequiredQueryParams returns true if the operation has required query parameters.
func hasRequiredQueryParams(op *ir.OperationDef) bool {
	return slices.ContainsFunc(op.QueryParams, func(p *ir.ParamDef) bool { return p.Required })
}

// hasRequiredHeaderParams returns true if the operation has required header parameters.
func hasRequiredHeaderParams(op *ir.OperationDef) bool {
	return slices.ContainsFunc(op.HeaderParams, func(p *ir.ParamDef) bool { return p.Required })
}

// hasRequiredCookieParams returns true if the operation has required cookie parameters.
func hasRequiredCookieParams(op *ir.OperationDef) bool {
	return slices.ContainsFunc(op.CookieParams, func(p *ir.ParamDef) bool { return p.Required })
}

// paramType returns the Go type expression for a parameter.
// For optional parameters in a params struct, pointer types are used.
func paramType(p *ir.ParamDef) string {
	if !p.Required {
		return "*" + p.Type
	}
	return p.Type
}

// hasUnions returns true if any of the given types is a union type.
func hasUnions(types []*ir.TypeDef) bool {
	for _, td := range types {
		if td.Kind == ir.TypeKindUnion {
			return true
		}
	}
	return false
}

// hasUntypedVariant reports whether a union has a variant no Go type could be
// derived for, whose payloads nothing but an any decode accepts.
func hasUntypedVariant(td *ir.TypeDef) bool {
	return slices.ContainsFunc(td.UnionTypes, func(v *ir.UnionVariant) bool { return v.TypeName == "any" })
}

// distinctVariants returns each variant Go type of a union once, so a type switch
// over them cannot repeat a case.
func distinctVariants(td *ir.TypeDef) []string {
	var names []string
	for _, v := range td.UnionTypes {
		if !slices.Contains(names, v.TypeName) {
			names = append(names, v.TypeName)
		}
	}
	return names
}

// unionBaseFields returns the fields a union's Base accessor copies out of the
// variant it holds. It is empty when the variants embed the base, which the
// accessor takes the address of instead.
func unionBaseFields(pkg *ir.Package, td *ir.TypeDef) []*ir.Field {
	if td.BaseType == "" || td.BaseEmbedded {
		return nil
	}
	base := typeIndex(pkg).byName[td.BaseType]
	if base == nil {
		return nil
	}
	return base.Fields
}

// primaryServer returns the server the spec presents first, which is the one the
// generated client defaults to. It returns nil when the spec declares none.
func primaryServer(pkg *ir.Package) *ir.ServerDef {
	if len(pkg.Servers) == 0 {
		return nil
	}
	return pkg.Servers[0]
}

// defaultBaseURL returns the primary server's URL with every template variable
// at its default, or "" when a variable has no default to substitute.
func defaultBaseURL(pkg *ir.Package) string {
	server := primaryServer(pkg)
	if server == nil {
		return ""
	}
	// A relative server URL is resolved against wherever the spec is served, which
	// a generated constant cannot stand in for.
	url := server.URL
	if !strings.Contains(url, "://") {
		return ""
	}
	for _, v := range server.Variables {
		if v.Default == "" {
			return ""
		}
		url = strings.ReplaceAll(url, "{"+v.Name+"}", v.Default)
	}
	return url
}

// InboundKind names one family of payloads the API sends: webhooks the document
// declares, or callbacks an operation registers.
type InboundKind struct {
	Callback bool
	Singular string
	Plural   string
	Suffix   string
}

// inboundKinds returns the families a package actually declares, so a spec with
// only webhooks generates nothing about callbacks.
func inboundKinds(pkg *ir.Package) []InboundKind {
	var kinds []InboundKind
	for _, kind := range []InboundKind{
		{Callback: false, Singular: "webhook", Plural: "Webhook", Suffix: "Webhook"},
		{Callback: true, Singular: "callback", Plural: "Callback", Suffix: "Callback"},
	} {
		if len(inboundPayloads(pkg, kind.Callback)) > 0 {
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

// inboundPayloads returns one family's payloads.
func inboundPayloads(pkg *ir.Package, callback bool) []*ir.WebhookDef {
	var defs []*ir.WebhookDef
	for _, w := range pkg.Webhooks {
		if w.Callback == callback {
			defs = append(defs, w)
		}
	}
	return defs

}

// serializedParam reports whether a parameter's value is written as a document in
// the media type it declares, rather than encoded under an OpenAPI style.
func serializedParam(p *ir.ParamDef) bool {
	return strings.Contains(p.ContentType, "json")

}

// reservedQueryParams returns the query parameters a spec marks allowReserved,
// whose values keep their reserved characters instead of being escaped.
func reservedQueryParams(op *ir.OperationDef) []*ir.ParamDef {
	var params []*ir.ParamDef
	for _, p := range op.QueryParams {
		if p.AllowReserved {
			params = append(params, p)
		}
	}
	return params
}

// operationHeaders returns the headers an operation declares across all of its
// responses, in declaration order and once each. The accessor hangs off the
// captured response, which may be an error response, so a header declared only
// on a 429 is as reachable as one declared on the 200.
func operationHeaders(op *ir.OperationDef) []*ir.ResponseHeaderDef {
	var headers []*ir.ResponseHeaderDef
	seen := map[string]bool{}
	for _, resp := range op.Responses {
		for _, h := range resp.Headers {
			key := strings.ToLower(h.Name)
			if seen[key] {
				continue
			}
			seen[key] = true
			headers = append(headers, h)
		}
	}
	return headers
}

// headerKinds returns the non-string header types the package parses, so only
// the parse helpers that are called get generated.
func headerKinds(pkg *ir.Package) []string {
	var kinds []string
	for _, kind := range []string{"int64", "float64", "bool"} {
		for _, op := range pkg.Operations {
			if slices.ContainsFunc(operationHeaders(op), func(h *ir.ResponseHeaderDef) bool { return h.Type == kind }) {
				kinds = append(kinds, kind)
				break
			}
		}
	}
	return kinds
}

// headerDocComment renders a header's description as a field doc comment.
func headerDocComment(h *ir.ResponseHeaderDef) string {
	return fieldDocComment(&ir.Field{Name: h.GoName, Description: h.Description})
}

// discriminatorFieldName converts a JSON property name to a Go field name
// for use in the discriminator struct in UnmarshalJSON.
func discriminatorFieldName(propertyName string) string {
	return naming.Exported(propertyName)
}

// hasPaginatedOps returns true if any operation has pagination configured.
func hasPaginatedOps(ops []*ir.OperationDef) bool {
	for _, op := range ops {
		if op.Pagination != nil {
			return true
		}
	}
	return false
}

// paginationItemType returns the element type for a paginated operation's items.
// The type is pre-computed during analysis and stored in Pagination.ItemsType.
func paginationItemType(op *ir.OperationDef) string {
	if op.Pagination == nil || op.Pagination.ItemsType == "" {
		return "any"
	}
	return op.Pagination.ItemsType
}

// paginationCursorField returns the Go struct field name (PascalCase) for the
// cursor parameter in the params struct.
func paginationCursorField(op *ir.OperationDef) string {
	if op.Pagination == nil {
		return ""
	}
	for _, p := range op.QueryParams {
		if p.OrigName == op.Pagination.CursorParam {
			return p.FieldName
		}
	}
	return naming.Exported(op.Pagination.CursorParam)
}

// uniqueErrorTypes returns deduplicated error response type names from all operations.
func uniqueErrorTypes(pkg *ir.Package) []ErrorWrapper {
	seen := map[string]bool{}
	var wrappers []ErrorWrapper
	for _, op := range pkg.Operations {
		for _, resp := range op.ErrorResponses {
			if resp.ErrorWrapper == "" || seen[resp.ErrorWrapper] {
				continue
			}
			seen[resp.ErrorWrapper] = true
			wrappers = append(wrappers, ErrorWrapper{Name: resp.ErrorWrapper, Detail: resp.TypeName})
		}
	}
	return wrappers
}

// ErrorWrapper is one generated error type: the name it declares and the type of
// the body it parses into.
type ErrorWrapper struct {
	Name   string
	Detail string
}

// errorMessageField returns the error type's string field annotated with
// x-ms-primary-error-message, or nil when the spec designates none.
func errorMessageField(pkg *ir.Package, typeName string) *ir.Field {
	for _, t := range pkg.Types {
		if t.Name != typeName || t.Kind != ir.TypeKindStruct {
			continue
		}
		for _, f := range t.Fields {
			if f.PrimaryErrorMessage && (f.Type == "string" || f.Type == "*string") {
				return f
			}
		}
	}
	return nil
}

// errorType returns the error response type name for an operation, or "".
func errorType(op *ir.OperationDef) string {
	for _, resp := range op.ErrorResponses {
		if resp.ErrorWrapper != "" {
			return resp.ErrorWrapper
		}
	}
	return ""
}

// successContentType returns the content type of the operation's success response.
// Defaults to "application/json" if no content type is set.
func successContentType(op *ir.OperationDef) string {
	if op.SuccessResponse == nil || op.SuccessResponse.ContentType == "" {
		return "application/json"
	}
	return op.SuccessResponse.ContentType
}

// requestContentType returns the media type an operation sends its request body
// as, or "" when it has no body.
func requestContentType(op *ir.OperationDef) string {
	if op.RequestBody == nil {
		return ""
	}
	return op.RequestBody.ContentType
}

// hasNonJSONBody reports whether any operation sends a request body in a media
// type other than JSON, which is what pulls the extra body encoders into the
// generated helpers.
func hasNonJSONBody(pkg *ir.Package) bool {
	return slices.ContainsFunc(pkg.Operations, func(op *ir.OperationDef) bool {
		ct := requestContentType(op)
		return ct != "" && !strings.Contains(ct, "json")
	})
}
