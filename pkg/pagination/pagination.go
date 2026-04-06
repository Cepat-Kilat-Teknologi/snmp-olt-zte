package pagination

import (
	"net/http"
	"strconv"
)

// Constants for default and maximum page sizes, and query parameter names
var (
	DefaultPageSize = 10      // Default number of items per page
	MaxPageSize     = 100     // Maximum allowed items per page
	PageVar         = "page"  // Query parameter key for page number
	PageSizeVar     = "limit" // Query parameter key for page size
)

// Pages struct defines the structure for pagination metadata.
// This structure is used to calculate and hold pagination parameters.
type Pages struct {
	Page      int `json:"page"`       // Current page number
	PageSize  int `json:"limit"`      // Number of items per page
	PageCount int `json:"page_count"` // Total number of pages
	TotalRows int `json:"total_rows"` // Total number of rows/items
}

// New creates a new Pages instance with the provided parameters
// It calculates the total page count and ensures page/pageSize are within valid bounds.
func New(page, pageSize, total int) *Pages {
	if page <= 0 { // Ensure page is at least 0 (or 1 depending on logic, here looks like 0)
		page = 0
	}
	if pageSize <= 0 { // Use default page size if invalid
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize { // Cap page size at maximum
		pageSize = MaxPageSize
	}
	pageCount := -1 // Default page count
	if total >= 0 { // Calculate page count if total is valid
		pageCount = (total + pageSize - 1) / pageSize
	}
	return &Pages{ // Return initialized Pages struct
		Page:      page,
		PageSize:  pageSize,
		TotalRows: total,
		PageCount: pageCount,
	}
}

// GetPaginationParametersFromRequest extracts pagination parameters from the HTTP request
// It parses 'page' and 'limit' query parameters, providing default values if missing or invalid.
func GetPaginationParametersFromRequest(r *http.Request) (pageIndex, pageSize int) {
	pageIndex = parseInt(r.URL.Query().Get(PageVar), 1)                  // Parse page number, default to 1
	pageSize = parseInt(r.URL.Query().Get(PageSizeVar), DefaultPageSize) // Parse page size, default to 10
	return pageIndex, pageSize                                           // Return extracted values
}

// parseInt is a helper function to parse string to int with a default value
// It handles empty strings and invalid numeric formats gracefully.
func parseInt(value string, defaultValue int) int {
	if value == "" { // If empty string, return default
		return defaultValue
	}
	if result, err := strconv.Atoi(value); err == nil { // Try to convert to integer
		return result // Return converted value on success
	}
	return defaultValue // Return default value on error
}
