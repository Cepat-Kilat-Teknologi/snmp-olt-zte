package utils

// WebResponse defines the standard API response structure.
// Meta is optional — only included for paginated responses.
type WebResponse struct {
	Code   int    `json:"code"`
	Status string `json:"status"`
	Data   any    `json:"data"`
	Meta   *Meta  `json:"meta,omitempty"`
}

// Meta contains pagination metadata.
type Meta struct {
	Page      int `json:"page"`
	Limit     int `json:"limit"`
	PageCount int `json:"page_count"`
	TotalRows int `json:"total_rows"`
}

// ErrorDetail contains structured error information.
type ErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// ErrorResponse defines the standard API error response structure.
type ErrorResponse struct {
	Code   int         `json:"code"`
	Status string      `json:"status"`
	Error  ErrorDetail `json:"error"`
}
