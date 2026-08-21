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

	items := findItemsField(ir.TypesByName(pkg.Types), respType)
	if items.name == "" {
		return nil
	}

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
		return a.buildOffsetPagination(op, pkg, ir.PaginationStyleOffset, offsetParam, limitParam)
	}

	// Check for page + (per_page | page_size | pageSize | limit) pattern.
	pageParam := ""
	if orig, ok := queryNames["page"]; ok {
		pageParam = orig
	}
	if pageParam == "" {
		return nil
	}

	// Both spellings of each name: the comparison is case-insensitive, so
	// perpage covers perPage and pagesize covers pageSize.
	for _, name := range []string{"per_page", "perpage", "page_size", "pagesize", "limit"} {
		if orig, ok := queryNames[name]; ok {
			return a.buildOffsetPagination(op, pkg, ir.PaginationStylePage, pageParam, orig)
		}
	}

	return nil
}

// buildOffsetPagination builds a PaginationDef for the styles that count rather
// than follow a cursor. The style decides how the parameter advances: an offset
// by the items received, a page by one.
func (a *Analyzer) buildOffsetPagination(op *ir.OperationDef, pkg *ir.Package, style ir.PaginationStyle, offsetParam, limitParam string) *ir.PaginationDef {
	respType := a.findSuccessResponseType(op, pkg)
	if respType == nil {
		return nil
	}
	items := findItemsField(ir.TypesByName(pkg.Types), respType)
	if items.name == "" {
		return nil
	}

	return &ir.PaginationDef{
		Style:       style,
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
// response type that contains the paginated items. It returns the zero value
// when nothing identifies one, which leaves the operation without an iterator
// rather than paging over whichever array the spec happens to declare first.
func findItemsField(byName map[string]*ir.TypeDef, td *ir.TypeDef) itemsFieldInfo {
	var arrays []*ir.Field
	for _, f := range td.Fields {
		if !strings.HasPrefix(f.Type, "[]") {
			continue
		}
		if containsCI(itemsFieldNames, f.JSONName) {
			return itemsFieldInfo{name: f.JSONName, elemType: f.Type[2:]}
		}
		arrays = append(arrays, f)
	}
	// One array is the page by elimination.
	if len(arrays) == 1 {
		return itemsFieldInfo{name: arrays[0].JSONName, elemType: arrays[0].Type[2:]}
	}
	// A page holds records, so an array of a declared type beside arrays of
	// scalars is still unambiguous. Two arrays of records are not, and guessing
	// there returns the wrong data instead of failing.
	if records := recordArrays(byName, arrays); len(records) == 1 {
		return itemsFieldInfo{name: records[0].JSONName, elemType: records[0].Type[2:]}
	}
	return itemsFieldInfo{}
}

// recordArrays returns the array fields whose element type is a type the spec
// declares rather than a builtin.
func recordArrays(byName map[string]*ir.TypeDef, arrays []*ir.Field) []*ir.Field {
	var records []*ir.Field
	for _, f := range arrays {
		if byName[ir.NamedType(f.Type[2:])] != nil {
			records = append(records, f)
		}
	}
	return records
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
