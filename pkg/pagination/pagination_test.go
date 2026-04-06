package pagination

import (
	"net/http/httptest"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name          string
		page          int
		pageSize      int
		total         int
		expectedPage  int
		expectedSize  int
		expectedCount int
	}{
		{
			name:          "Valid pagination - page 1",
			page:          1,
			pageSize:      10,
			total:         100,
			expectedPage:  1,
			expectedSize:  10,
			expectedCount: 10,
		},
		{
			name:          "Valid pagination - page 5",
			page:          5,
			pageSize:      20,
			total:         150,
			expectedPage:  5,
			expectedSize:  20,
			expectedCount: 8, // ceil(150/20) = 8
		},
		{
			name:          "Zero page - should default to 0",
			page:          0,
			pageSize:      10,
			total:         100,
			expectedPage:  0,
			expectedSize:  10,
			expectedCount: 10,
		},
		{
			name:          "Negative page - should default to 0",
			page:          -1,
			pageSize:      10,
			total:         100,
			expectedPage:  0,
			expectedSize:  10,
			expectedCount: 10,
		},
		{
			name:          "Zero pageSize - should use default (10)",
			page:          1,
			pageSize:      0,
			total:         100,
			expectedPage:  1,
			expectedSize:  DefaultPageSize,
			expectedCount: 10,
		},
		{
			name:          "Negative pageSize - should use default (10)",
			page:          1,
			pageSize:      -5,
			total:         100,
			expectedPage:  1,
			expectedSize:  DefaultPageSize,
			expectedCount: 10,
		},
		{
			name:          "PageSize exceeds max - should cap at MaxPageSize",
			page:          1,
			pageSize:      200,
			total:         500,
			expectedPage:  1,
			expectedSize:  MaxPageSize,
			expectedCount: 5, // ceil(500/100) = 5
		},
		{
			name:          "Negative total - pageCount should be -1",
			page:          1,
			pageSize:      10,
			total:         -1,
			expectedPage:  1,
			expectedSize:  10,
			expectedCount: -1,
		},
		{
			name:          "Total is zero",
			page:          1,
			pageSize:      10,
			total:         0,
			expectedPage:  1,
			expectedSize:  10,
			expectedCount: 0,
		},
		{
			name:          "Total less than pageSize",
			page:          1,
			pageSize:      10,
			total:         5,
			expectedPage:  1,
			expectedSize:  10,
			expectedCount: 1, // ceil(5/10) = 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := New(tt.page, tt.pageSize, tt.total)

			if result.Page != tt.expectedPage {
				t.Errorf("Expected page %d, got %d", tt.expectedPage, result.Page)
			}

			if result.PageSize != tt.expectedSize {
				t.Errorf("Expected pageSize %d, got %d", tt.expectedSize, result.PageSize)
			}

			if result.PageCount != tt.expectedCount {
				t.Errorf("Expected pageCount %d, got %d", tt.expectedCount, result.PageCount)
			}

			if result.TotalRows != tt.total {
				t.Errorf("Expected totalRows %d, got %d", tt.total, result.TotalRows)
			}
		})
	}
}

func TestGetPaginationParametersFromRequest(t *testing.T) {
	tests := []struct {
		name             string
		pageParam        string
		limitParam       string
		expectedPage     int
		expectedPageSize int
	}{
		{
			name:             "Valid parameters",
			pageParam:        "2",
			limitParam:       "20",
			expectedPage:     2,
			expectedPageSize: 20,
		},
		{
			name:             "Missing page parameter - should default to 1",
			pageParam:        "",
			limitParam:       "20",
			expectedPage:     1,
			expectedPageSize: 20,
		},
		{
			name:             "Missing limit parameter - should use default",
			pageParam:        "3",
			limitParam:       "",
			expectedPage:     3,
			expectedPageSize: DefaultPageSize,
		},
		{
			name:             "Missing both parameters - should use defaults",
			pageParam:        "",
			limitParam:       "",
			expectedPage:     1,
			expectedPageSize: DefaultPageSize,
		},
		{
			name:             "Invalid page parameter - should default to 1",
			pageParam:        "abc",
			limitParam:       "20",
			expectedPage:     1,
			expectedPageSize: 20,
		},
		{
			name:             "Invalid limit parameter - should use default",
			pageParam:        "2",
			limitParam:       "xyz",
			expectedPage:     2,
			expectedPageSize: DefaultPageSize,
		},
		{
			name:             "Negative page - parsed as is",
			pageParam:        "-1",
			limitParam:       "20",
			expectedPage:     -1,
			expectedPageSize: 20,
		},
		{
			name:             "Zero page",
			pageParam:        "0",
			limitParam:       "20",
			expectedPage:     0,
			expectedPageSize: 20,
		},
		{
			name:             "Large values",
			pageParam:        "100",
			limitParam:       "50",
			expectedPage:     100,
			expectedPageSize: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request with query parameters
			req := httptest.NewRequest("GET", "/test", nil)
			q := req.URL.Query()
			if tt.pageParam != "" {
				q.Add(PageVar, tt.pageParam)
			}
			if tt.limitParam != "" {
				q.Add(PageSizeVar, tt.limitParam)
			}
			req.URL.RawQuery = q.Encode()

			page, pageSize := GetPaginationParametersFromRequest(req)

			if page != tt.expectedPage {
				t.Errorf("Expected page %d, got %d", tt.expectedPage, page)
			}

			if pageSize != tt.expectedPageSize {
				t.Errorf("Expected pageSize %d, got %d", tt.expectedPageSize, pageSize)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		defaultValue int
		expected     int
	}{
		{
			name:         "Valid integer",
			value:        "42",
			defaultValue: 10,
			expected:     42,
		},
		{
			name:         "Empty string - use default",
			value:        "",
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "Invalid string - use default",
			value:        "abc",
			defaultValue: 5,
			expected:     5,
		},
		{
			name:         "Negative integer",
			value:        "-5",
			defaultValue: 10,
			expected:     -5,
		},
		{
			name:         "Zero",
			value:        "0",
			defaultValue: 10,
			expected:     0,
		},
		{
			name:         "Large number",
			value:        "999999",
			defaultValue: 10,
			expected:     999999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseInt(tt.value, tt.defaultValue)

			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}
