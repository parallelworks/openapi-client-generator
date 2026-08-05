package generator

import (
	"slices"
	"strings"
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
		"hasOperations":           hasOperations,
		"successType":             successType,
		"hasBody":                 hasBody,
		"hasRequiredQueryParams":  hasRequiredQueryParams,
		"hasRequiredHeaderParams": hasRequiredHeaderParams,
		"hasRequiredCookieParams": hasRequiredCookieParams,
		"paramType":               paramType,
		"hasUnions":               hasUnions,
		"catchAllField":           catchAllField,
		"catchAllValueType":       catchAllValueType,
		"declaredJSONNames":       declaredJSONNames,
		"hasCatchAllTypes":        hasCatchAllTypes,
		"discriminatorFieldName":  discriminatorFieldName,
		"hasPaginatedOps":         hasPaginatedOps,
		"paginationItemType":      paginationItemType,
		"paginationCursorField":   paginationCursorField,
		"toGoName":                naming.Exported,
		"uniqueErrorTypes":        uniqueErrorTypes,
		"errorMessageField":       errorMessageField,
		"errorType":               errorType,
		"successContentType":      successContentType,
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

func declaredJSONNames(td *ir.TypeDef) []string {
	var names []string
	for _, f := range td.Fields {
		if f.CatchAll || f.Embedded || f.JSONName == "" || f.JSONName == "-" {
			continue
		}
		names = append(names, f.JSONName)
	}
	return names
}

func hasCatchAllTypes(types []*ir.TypeDef) bool {
	return slices.ContainsFunc(types, func(td *ir.TypeDef) bool {
		return catchAllField(td) != nil
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
func uniqueErrorTypes(pkg *ir.Package) []string {
	seen := map[string]bool{}
	var types []string
	for _, op := range pkg.Operations {
		for _, resp := range op.ErrorResponses {
			if resp.TypeName != "" && !seen[resp.TypeName] {
				seen[resp.TypeName] = true
				types = append(types, resp.TypeName)
			}
		}
	}
	return types
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
		if resp.TypeName != "" {
			return resp.TypeName
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
