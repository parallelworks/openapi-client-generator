package analyzer

import (
	"strings"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
)

// cursorParamNames are query parameter names that indicate cursor-based pagination.
var cursorParamNames = []string{
	"cursor", "after", "page_token", "pageToken", "next_token", "nextToken",
}

// cursorFieldNames are response field names that contain the next cursor value.
var cursorFieldNames = []string{
	"next_cursor", "nextCursor", "next_page_token", "nextPageToken", "cursor",
}

// itemsFieldNames are response field names that contain the paginated items array.
var itemsFieldNames = []string{
	"items", "data", "results",
}

// detectPagination detects paginated operations and sets their Pagination field.
func (a *Analyzer) detectPagination(pkg *ir.Package) {
	for _, op := range pkg.Operations {
		if pd := a.detectCursorPagination(op, pkg); pd != nil {
			op.Pagination = pd
			continue
		}
		if pd := a.detectOffsetPagination(op, pkg); pd != nil {
			op.Pagination = pd
		}
	}
}

// detectCursorPagination checks if an operation uses cursor-based pagination.
func (a *Analyzer) detectCursorPagination(op *ir.OperationDef, pkg *ir.Package) *ir.PaginationDef {
	// The cursor param must be optional: the iterator drives it as a pointer field
	// it sets each page, so a required (value) cursor can't be paginated.
	cursorParam := ""
	for _, p := range op.QueryParams {
		if !p.Required && containsCI(cursorParamNames, p.OrigName) {
			cursorParam = p.OrigName
			break
		}
	}
	if cursorParam == "" {
		return nil
	}

	// Find the success response type and look for a cursor field.
	respType := a.findSuccessResponseType(op, pkg)
	if respType == nil {
		return nil
	}

	cursorField := ""
	for _, f := range respType.Fields {
		if containsCI(cursorFieldNames, f.JSONName) {
			cursorField = f.JSONName
			break
		}
	}
	if cursorField == "" {
		return nil
	}

	// Find the items field.
	items := findItemsField(respType)

	return &ir.PaginationDef{
		Style:       ir.PaginationStyleCursor,
		CursorParam: cursorParam,
		CursorField: cursorField,
		ItemsField:  items.name,
		ItemsType:   items.elemType,
	}
}

// detectOffsetPagination checks if an operation uses offset-based pagination.
func (a *Analyzer) detectOffsetPagination(op *ir.OperationDef, pkg *ir.Package) *ir.PaginationDef {
	queryNames := make(map[string]string) // lowercase -> origName
	for _, p := range op.QueryParams {
		queryNames[strings.ToLower(p.OrigName)] = p.OrigName
	}

	offsetParam := ""
	limitParam := ""

	// Check for offset + limit pattern.
	if orig, ok := queryNames["offset"]; ok {
		offsetParam = orig
	}
	if orig, ok := queryNames["limit"]; ok {
		limitParam = orig
	}
	if offsetParam != "" && limitParam != "" {
		return a.buildOffsetPagination(op, pkg, offsetParam, limitParam)
	}

	// Check for page + (per_page | page_size | pageSize | limit) pattern.
	pageParam := ""
	if orig, ok := queryNames["page"]; ok {
		pageParam = orig
	}
	if pageParam == "" {
		return nil
	}

	for _, name := range []string{"per_page", "page_size", "pagesize", "limit"} {
		if orig, ok := queryNames[name]; ok {
			return a.buildOffsetPagination(op, pkg, pageParam, orig)
		}
	}

	return nil
}

// buildOffsetPagination builds an offset-style PaginationDef.
func (a *Analyzer) buildOffsetPagination(op *ir.OperationDef, pkg *ir.Package, offsetParam, limitParam string) *ir.PaginationDef {
	respType := a.findSuccessResponseType(op, pkg)
	var items itemsFieldInfo
	if respType != nil {
		items = findItemsField(respType)
	}

	return &ir.PaginationDef{
		Style:       ir.PaginationStyleOffset,
		OffsetParam: offsetParam,
		LimitParam:  limitParam,
		ItemsField:  items.name,
		ItemsType:   items.elemType,
	}
}

// findSuccessResponseType finds the TypeDef for the success response of an operation.
func (a *Analyzer) findSuccessResponseType(op *ir.OperationDef, pkg *ir.Package) *ir.TypeDef {
	if op.SuccessResponse == nil || op.SuccessResponse.TypeName == "" {
		return nil
	}
	for _, td := range pkg.Types {
		if td.Name == op.SuccessResponse.TypeName {
			return td
		}
	}
	return nil
}

// itemsFieldInfo holds the name and element type of a paginated items field.
type itemsFieldInfo struct {
	name     string
	elemType string
}

// findItemsField finds the name and element type of the array-typed field in a
// response type that contains the paginated items.
func findItemsField(td *ir.TypeDef) itemsFieldInfo {
	for _, f := range td.Fields {
		if containsCI(itemsFieldNames, f.JSONName) && strings.HasPrefix(f.Type, "[]") {
			return itemsFieldInfo{name: f.JSONName, elemType: f.Type[2:]}
		}
	}
	// Fallback: find any array-typed field.
	for _, f := range td.Fields {
		if strings.HasPrefix(f.Type, "[]") {
			return itemsFieldInfo{name: f.JSONName, elemType: f.Type[2:]}
		}
	}
	return itemsFieldInfo{}
}

// containsCI checks if any element in the list matches the target (case-insensitive).
func containsCI(list []string, target string) bool {
	lower := strings.ToLower(target)
	for _, s := range list {
		if strings.ToLower(s) == lower {
			return true
		}
	}
	return false
}
