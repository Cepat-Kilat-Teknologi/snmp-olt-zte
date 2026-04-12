package utils

// WebResponse defines the standard API response structure for success responses.
// Meta is optional — only included for paginated responses.
//
// Format:
//
//	{"code":200, "status":"success", "data":..., "meta":{...}}
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

// ErrorResponse defines the standard API error response structure.
//
// Format:
//
//	{"code":400, "status":"Bad Request", "error_code":"VALIDATION_ERROR", "data":..., "request_id":"..."}
//
// The `data` field contains either the error message (string) or structured
// validation details (object/array).
type ErrorResponse struct {
	Code      int    `json:"code"`
	Status    string `json:"status"`
	ErrorCode string `json:"error_code"`
	Data      any    `json:"data"`
	RequestID string `json:"request_id,omitempty"`
}
