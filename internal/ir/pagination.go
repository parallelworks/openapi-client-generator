package ir

// PaginationStyle represents the pagination strategy.
type PaginationStyle int

const (
	PaginationStyleCursor PaginationStyle = iota
	PaginationStyleOffset
)

// PaginationDef describes pagination for an operation.
type PaginationDef struct {
	Style PaginationStyle
	// For cursor-based:
	CursorParam string // Query param name for the cursor
	CursorField string // Response field containing next cursor
	// For offset-based:
	OffsetParam string
	LimitParam  string
	// Common:
	HasMoreField string // Response field indicating more pages exist (optional)
	TotalField   string // Response field with total count (optional)
	ItemsField   string // Response field containing the items array
	ItemsType    string // Go element type of the items array (e.g., "Pet")
}
