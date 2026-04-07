package handler

import (
	"net/http"

	apperrors "github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/errors"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/middleware"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/usecase"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/utils"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/pagination"
	"github.com/rs/zerolog/log"
)

// getRequestID extracts the request ID from the request header
func getRequestID(r *http.Request) string {
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}
	return ""
}

// OnuHandlerInterface is an interface that represents the auth's handler contract
type OnuHandlerInterface interface {
	GetByBoardIDAndPonID(w http.ResponseWriter, r *http.Request)             // Handler to get ONU info by board and PON
	GetByBoardIDPonIDAndOnuID(w http.ResponseWriter, r *http.Request)        // Handler to get specific ONU info
	GetEmptyOnuID(w http.ResponseWriter, r *http.Request)                    // Handler to get empty ONU IDs
	GetOnuIDAndSerialNumber(w http.ResponseWriter, r *http.Request)          // Handler to get ONU IDs and serial numbers
	UpdateEmptyOnuID(w http.ResponseWriter, r *http.Request)                 // Handler to update empty ONU IDs
	GetByBoardIDAndPonIDWithPaginate(w http.ResponseWriter, r *http.Request) // Handler to get paginated ONU info
	DeleteCache(w http.ResponseWriter, r *http.Request)                      // Handler to delete cache for board/pon
}

// OnuHandler is a struct that represents the auth handler
type OnuHandler struct {
	ponUsecase usecase.OnuUseCaseInterface // Usecase interface dependency
}

// NewOnuHandler will create an object that represents the auth handler
func NewOnuHandler(ponUsecase usecase.OnuUseCaseInterface) *OnuHandler {
	return &OnuHandler{ponUsecase: ponUsecase} // Return new OnuHandler with injected usecase
}

// GetByBoardIDAndPonID is a method to get one info by board id and pon id
// example: http://localhost:8080/api/v1/board/1/pon/1
func (o *OnuHandler) GetByBoardIDAndPonID(w http.ResponseWriter, r *http.Request) {
	// Get pre-validated values from context
	boardIDInt, _ := middleware.GetBoardID(r.Context()) // Retrieve boardID from context
	ponIDInt, _ := middleware.GetPonID(r.Context())     // Retrieve ponID from context

	log.Info().
		Str("request_id", getRequestID(r)).
		Int("board_id", boardIDInt).
		Int("pon_id", ponIDInt).
		Msg("Getting ONU info by board and PON") // Log the request

	query := r.URL.Query() // Get query parameters

	// Validate query parameters and return error 400 if query parameters is not "onu_id" or empty query parameters
	if len(query) > 0 && query["onu_id"] == nil {
		log.Warn().Interface("query_parameters", query).Msg("Invalid query parameter") // Log warning
		appErr := apperrors.NewValidationError(
			"invalid query parameter - only 'onu_id' is allowed",
			map[string]interface{}{"received": query},
		) // Create validation error
		utils.HandleError(w, appErr) // Handle and respond with error
		return
	}

	// Call usecase to get data from SNMP
	onuInfoList, err := o.ponUsecase.GetByBoardIDAndPonID(r.Context(), boardIDInt, ponIDInt)
	if err != nil {
		log.Error().
			Err(err).
			Int("board_id", boardIDInt).
			Int("pon_id", ponIDInt).
			Msg("Failed to get ONU info from SNMP") // Log error
		utils.HandleError(w, err) // Handle error
		return
	}

	// Check if the result list is empty
	if len(onuInfoList) == 0 {
		log.Warn().
			Int("board_id", boardIDInt).
			Int("pon_id", ponIDInt).
			Msg("ONU info not found") // Log warning
		appErr := apperrors.NewNotFoundError("ONU info",
			map[string]int{"board_id": boardIDInt, "pon_id": ponIDInt}) // Create not found error
		utils.HandleError(w, appErr) // Handle error
		return
	}

	log.Info().
		Int("board_id", boardIDInt).
		Int("pon_id", ponIDInt).
		Int("result_count", len(onuInfoList)).
		Msg("Successfully retrieved ONU info") // Log success

	// Create web response object
	response := utils.WebResponse{
		Code:   http.StatusOK, // Status 200
		Status: "OK",
		Data:   onuInfoList, // Payload
	}

	utils.SendJSONResponse(w, http.StatusOK, response) // Send JSON response
}

// GetByBoardIDPonIDAndOnuID is a method to get one info by board id, pon id, and onu id
// example: http://localhost:8080/api/v1/board/1/pon/1/onu/1
func (o *OnuHandler) GetByBoardIDPonIDAndOnuID(w http.ResponseWriter, r *http.Request) {
	// Get pre-validated values from context
	boardIDInt, _ := middleware.GetBoardID(r.Context()) // Get boardID
	ponIDInt, _ := middleware.GetPonID(r.Context())     // Get ponID
	onuIDInt, _ := middleware.GetOnuID(r.Context())     // Get onuID

	log.Info().
		Str("request_id", getRequestID(r)).
		Int("board_id", boardIDInt).
		Int("pon_id", ponIDInt).
		Int("onu_id", onuIDInt).
		Msg("Getting specific ONU info") // Log request

	// Call usecase to get data from SNMP
	onuInfoList, err := o.ponUsecase.GetByBoardIDPonIDAndOnuID(r.Context(), boardIDInt, ponIDInt, onuIDInt)
	if err != nil {
		log.Error().
			Err(err).
			Int("board_id", boardIDInt).
			Int("pon_id", ponIDInt).
			Int("onu_id", onuIDInt).
			Msg("Failed to get specific ONU info from SNMP") // Log error
		utils.HandleError(w, err) // Handle error
		return
	}

	// Check if the returned object is empty (default zero values)
	if onuInfoList.Board == 0 && onuInfoList.PON == 0 && onuInfoList.ID == 0 {
		log.Warn().
			Int("board_id", boardIDInt).
			Int("pon_id", ponIDInt).
			Int("onu_id", onuIDInt).
			Msg("ONU not found") // Log warning
		appErr := apperrors.NewNotFoundError("ONU",
			map[string]int{"board_id": boardIDInt, "pon_id": ponIDInt, "onu_id": onuIDInt}) // Create not found error
		utils.HandleError(w, appErr) // Handle error
		return
	}

	log.Info().
		Int("board_id", boardIDInt).
		Int("pon_id", ponIDInt).
		Int("onu_id", onuIDInt).
		Msg("Successfully retrieved specific ONU info") // Log success

	// Create a web response
	response := utils.WebResponse{
		Code:   http.StatusOK,
		Status: "OK",
		Data:   onuInfoList,
	}

	utils.SendJSONResponse(w, http.StatusOK, response) // Send JSON response
}

// GetEmptyOnuID is a method to get empty onu id by board id and pon id
// example: http://localhost:8080/api/v1/board/1/pon/1/onu_id/empty
func (o *OnuHandler) GetEmptyOnuID(w http.ResponseWriter, r *http.Request) {
	// Get pre-validated values from context
	boardIDInt, _ := middleware.GetBoardID(r.Context())
	ponIDInt, _ := middleware.GetPonID(r.Context())

	log.Info().
		Str("request_id", getRequestID(r)).
		Int("board_id", boardIDInt).
		Int("pon_id", ponIDInt).
		Msg("Getting empty ONU IDs") // Log request

	// Call usecase to get data from SNMP
	onuIDEmptyList, err := o.ponUsecase.GetEmptyOnuID(r.Context(), boardIDInt, ponIDInt)
	if err != nil {
		log.Error().
			Err(err).
			Int("board_id", boardIDInt).
			Int("pon_id", ponIDInt).
			Msg("Failed to get empty ONU IDs from SNMP") // Log error
		utils.HandleError(w, err)
		return
	}

	log.Info().
		Int("board_id", boardIDInt).
		Int("pon_id", ponIDInt).
		Int("empty_count", len(onuIDEmptyList)).
		Msg("Successfully retrieved empty ONU IDs") // Log success

	// Create a web response
	response := utils.WebResponse{
		Code:   http.StatusOK,
		Status: "OK",
		Data:   onuIDEmptyList,
	}

	utils.SendJSONResponse(w, http.StatusOK, response) // Send JSON response
}

// GetOnuIDAndSerialNumber is a method to get onu id and serial number by board id and pon id
// example: http://localhost:8080/api/v1/board/1/pon/1/onu_id_sn
func (o *OnuHandler) GetOnuIDAndSerialNumber(w http.ResponseWriter, r *http.Request) {
	// Get pre-validated values from context
	boardIDInt, _ := middleware.GetBoardID(r.Context())
	ponIDInt, _ := middleware.GetPonID(r.Context())

	log.Info().
		Str("request_id", getRequestID(r)).
		Int("board_id", boardIDInt).
		Int("pon_id", ponIDInt).
		Msg("Getting ONU IDs and serial numbers") // Log request

	// Call usecase to get Serial Number from SNMP
	onuSerialNumber, err := o.ponUsecase.GetOnuIDAndSerialNumber(r.Context(), boardIDInt, ponIDInt)
	if err != nil {
		log.Error().
			Err(err).
			Int("board_id", boardIDInt).
			Int("pon_id", ponIDInt).
			Msg("Failed to get ONU serial numbers from SNMP") // Log error
		utils.HandleError(w, err)
		return
	}

	log.Info().
		Int("board_id", boardIDInt).
		Int("pon_id", ponIDInt).
		Int("result_count", len(onuSerialNumber)).
		Msg("Successfully retrieved ONU serial numbers") // Log success

	// Create a web response
	response := utils.WebResponse{
		Code:   http.StatusOK,
		Status: "OK",
		Data:   onuSerialNumber,
	}

	utils.SendJSONResponse(w, http.StatusOK, response) // Send JSON response
}

// UpdateEmptyOnuID is a method to update empty onu id by board id and pon id
// example: http://localhost:8080/api/v1/board/1/pon/1/onu_id/update
func (o *OnuHandler) UpdateEmptyOnuID(w http.ResponseWriter, r *http.Request) {
	// Get pre-validated values from context
	boardIDInt, _ := middleware.GetBoardID(r.Context())
	ponIDInt, _ := middleware.GetPonID(r.Context())

	log.Info().
		Str("request_id", getRequestID(r)).
		Int("board_id", boardIDInt).
		Int("pon_id", ponIDInt).
		Msg("Updating empty ONU IDs") // Log request

	// Call usecase to get data from SNMP
	err := o.ponUsecase.UpdateEmptyOnuID(r.Context(), boardIDInt, ponIDInt)
	if err != nil {
		log.Error().
			Err(err).
			Int("board_id", boardIDInt).
			Int("pon_id", ponIDInt).
			Msg("Failed to update empty ONU IDs") // Log error
		utils.HandleError(w, err)
		return
	}

	log.Info().
		Int("board_id", boardIDInt).
		Int("pon_id", ponIDInt).
		Msg("Successfully updated empty ONU IDs") // Log success

	// Create a web response
	response := utils.WebResponse{
		Code:   http.StatusOK,
		Status: "OK",
		Data:   "Success Update Empty ONU_ID",
	}

	utils.SendJSONResponse(w, http.StatusOK, response) // Send JSON response
}

// GetByBoardIDAndPonIDWithPaginate is a method to get one info by board id and pon id with pagination
// example: http://localhost:8080/api/v1/paginate/board/1/pon/1?page=1&page_size=10
func (o *OnuHandler) GetByBoardIDAndPonIDWithPaginate(w http.ResponseWriter, r *http.Request) {
	// Get pre-validated values from context
	boardIDInt, _ := middleware.GetBoardID(r.Context())
	ponIDInt, _ := middleware.GetPonID(r.Context())

	// Get page and page size parameters from the request
	pageIndex, pageSize := pagination.GetPaginationParametersFromRequest(r)

	log.Info().
		Str("request_id", getRequestID(r)).
		Int("board_id", boardIDInt).
		Int("pon_id", ponIDInt).
		Int("page", pageIndex).
		Int("page_size", pageSize).
		Msg("Getting paginated ONU info") // Log request

	// Call usecase to get paginated data
	item, count := o.ponUsecase.GetByBoardIDAndPonIDWithPagination(r.Context(), boardIDInt, ponIDInt, pageIndex, pageSize)

	// Check if no items found
	if len(item) == 0 {
		log.Warn().
			Int("board_id", boardIDInt).
			Int("pon_id", ponIDInt).
			Int("page", pageIndex).
			Msg("No ONU data found for page") // Log warning
		appErr := apperrors.NewNotFoundError("ONU data",
			map[string]interface{}{
				"board_id": boardIDInt,
				"pon_id":   ponIDInt,
				"page":     pageIndex,
			}) // Create not found error
		utils.HandleError(w, appErr) // Handle error
		return
	}

	// Convert result to JSON format according to Pages structure
	pages := pagination.New(pageIndex, pageSize, count) // Create pagination meta data

	log.Info().
		Int("board_id", boardIDInt).
		Int("pon_id", ponIDInt).
		Int("page", pageIndex).
		Int("page_size", pageSize).
		Int("total_rows", pages.TotalRows).
		Int("page_count", pages.PageCount).
		Msg("Successfully retrieved paginated ONU info") // Log success

	// Create pagination response
	response := utils.WebResponse{
		Code:   http.StatusOK,
		Status: "OK",
		Data:   item,
		Meta: &utils.Meta{
			Page:      pages.Page,
			Limit:     pages.PageSize,
			PageCount: pages.PageCount,
			TotalRows: pages.TotalRows,
		},
	}

	utils.SendJSONResponse(w, http.StatusOK, response) // Send JSON response
}

// DeleteCache is a handler to delete cache for specific board and PON
// example: DELETE http://localhost:8081/api/v1/board/1/pon/1/cache/clear
func (o *OnuHandler) DeleteCache(w http.ResponseWriter, r *http.Request) {
	// Get pre-validated values from context
	boardIDInt, _ := middleware.GetBoardID(r.Context())
	ponIDInt, _ := middleware.GetPonID(r.Context())

	log.Info().
		Str("request_id", getRequestID(r)).
		Int("board_id", boardIDInt).
		Int("pon_id", ponIDInt).
		Msg("Deleting cache for board/pon")

	// Call usecase to delete cache
	err := o.ponUsecase.DeleteCache(r.Context(), boardIDInt, ponIDInt)
	if err != nil {
		log.Error().Err(err).
			Int("board_id", boardIDInt).
			Int("pon_id", ponIDInt).
			Msg("Failed to delete cache")
		utils.HandleError(w, err)
		return
	}

	log.Info().
		Int("board_id", boardIDInt).
		Int("pon_id", ponIDInt).
		Msg("Successfully deleted cache")

	// Send success response
	utils.SendJSONResponse(w, http.StatusOK, utils.WebResponse{
		Code:   http.StatusOK,
		Status: "OK",
		Data: map[string]interface{}{
			"message":  "Cache deleted successfully",
			"board_id": boardIDInt,
			"pon_id":   ponIDInt,
		},
	})
}
