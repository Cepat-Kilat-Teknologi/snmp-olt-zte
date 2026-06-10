package handler

import (
	"net/http"

	apperrors "github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/errors"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/middleware"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/usecase"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/utils"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/pagination"
	"go.uber.org/zap"
)

// OnuHandlerInterface is an interface that represents the auth's handler contract
type OnuHandlerInterface interface {
	GetByBoardIDAndPonID(w http.ResponseWriter, r *http.Request)             // Handler to get ONU info by board and PON
	GetByBoardIDPonIDAndOnuID(w http.ResponseWriter, r *http.Request)        // Handler to get specific ONU info
	GetEmptyOnuID(w http.ResponseWriter, r *http.Request)                    // Handler to get empty ONU IDs
	GetOnuIDAndSerialNumber(w http.ResponseWriter, r *http.Request)          // Handler to get ONU IDs and serial numbers
	UpdateEmptyOnuID(w http.ResponseWriter, r *http.Request)                 // Handler to update empty ONU IDs
	GetByBoardIDAndPonIDWithPaginate(w http.ResponseWriter, r *http.Request) // Handler to get paginated ONU info
	DeleteCache(w http.ResponseWriter, r *http.Request)                      // Handler to delete cache for board/pon
	InvalidateOnuCache(w http.ResponseWriter, r *http.Request)               // Handler to invalidate list + detail cache for one ONU
	GetUplinkTopology(w http.ResponseWriter, r *http.Request)                // Handler to auto-detect cards + uplink ports via SNMP
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

	log := logger.WithRequestID(r.Context())

	log.Info("getting_onu_info_by_board_and_pon",
		zap.Int("board_id", boardIDInt),
		zap.Int("pon_id", ponIDInt),
	)

	query := r.URL.Query() // Get query parameters

	// Validate query parameters and return error 400 if query parameters is not "onu_id" or empty query parameters
	if len(query) > 0 && query["onu_id"] == nil {
		log.Warn("invalid_query_parameter", zap.Any("query_parameters", query))
		appErr := apperrors.NewValidationError(
			"invalid query parameter - only 'onu_id' is allowed",
			map[string]interface{}{"received": query},
		) // Create validation error
		utils.HandleError(w, r, appErr) // Handle and respond with error
		return
	}

	// Call usecase to get data from SNMP
	onuInfoList, err := o.ponUsecase.GetByBoardIDAndPonID(r.Context(), boardIDInt, ponIDInt)
	if err != nil {
		log.Error("failed_to_get_onu_info_from_snmp",
			zap.Error(err),
			zap.Int("board_id", boardIDInt),
			zap.Int("pon_id", ponIDInt),
		)
		utils.HandleError(w, r, err) // Handle error
		return
	}

	// Check if the result list is empty
	if len(onuInfoList) == 0 {
		log.Warn("onu_info_not_found",
			zap.Int("board_id", boardIDInt),
			zap.Int("pon_id", ponIDInt),
		)
		appErr := apperrors.NewNotFoundError("ONU info",
			map[string]int{"board_id": boardIDInt, "pon_id": ponIDInt}) // Create not found error
		utils.HandleError(w, r, appErr) // Handle error
		return
	}

	log.Info("successfully_retrieved_onu_info",
		zap.Int("board_id", boardIDInt),
		zap.Int("pon_id", ponIDInt),
		zap.Int("result_count", len(onuInfoList)),
	)

	// Create web response object
	response := utils.WebResponse{
		Code:   http.StatusOK, // Status 200
		Status: "success",
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

	log := logger.WithRequestID(r.Context())

	log.Info("getting_specific_onu_info",
		zap.Int("board_id", boardIDInt),
		zap.Int("pon_id", ponIDInt),
		zap.Int("onu_id", onuIDInt),
	)

	// Call usecase to get data from SNMP
	onuInfoList, err := o.ponUsecase.GetByBoardIDPonIDAndOnuID(r.Context(), boardIDInt, ponIDInt, onuIDInt)
	if err != nil {
		log.Error("failed_to_get_specific_onu_info_from_snmp",
			zap.Error(err),
			zap.Int("board_id", boardIDInt),
			zap.Int("pon_id", ponIDInt),
			zap.Int("onu_id", onuIDInt),
		)
		utils.HandleError(w, r, err) // Handle error
		return
	}

	// Check if the returned object is empty (default zero values)
	if onuInfoList.Board == 0 && onuInfoList.PON == 0 && onuInfoList.ID == 0 {
		log.Warn("onu_not_found",
			zap.Int("board_id", boardIDInt),
			zap.Int("pon_id", ponIDInt),
			zap.Int("onu_id", onuIDInt),
		)
		appErr := apperrors.NewNotFoundError("ONU",
			map[string]int{"board_id": boardIDInt, "pon_id": ponIDInt, "onu_id": onuIDInt}) // Create not found error
		utils.HandleError(w, r, appErr) // Handle error
		return
	}

	log.Info("successfully_retrieved_specific_onu_info",
		zap.Int("board_id", boardIDInt),
		zap.Int("pon_id", ponIDInt),
		zap.Int("onu_id", onuIDInt),
	)

	// Create a web response
	response := utils.WebResponse{
		Code:   http.StatusOK,
		Status: "success",
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

	log := logger.WithRequestID(r.Context())

	log.Info("getting_empty_onu_ids",
		zap.Int("board_id", boardIDInt),
		zap.Int("pon_id", ponIDInt),
	)

	// Call usecase to get data from SNMP
	onuIDEmptyList, err := o.ponUsecase.GetEmptyOnuID(r.Context(), boardIDInt, ponIDInt)
	if err != nil {
		log.Error("failed_to_get_empty_onu_ids_from_snmp",
			zap.Error(err),
			zap.Int("board_id", boardIDInt),
			zap.Int("pon_id", ponIDInt),
		)
		utils.HandleError(w, r, err)
		return
	}

	log.Info("successfully_retrieved_empty_onu_ids",
		zap.Int("board_id", boardIDInt),
		zap.Int("pon_id", ponIDInt),
		zap.Int("empty_count", len(onuIDEmptyList)),
	)

	// Create a web response
	response := utils.WebResponse{
		Code:   http.StatusOK,
		Status: "success",
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

	log := logger.WithRequestID(r.Context())

	log.Info("getting_onu_ids_and_serial_numbers",
		zap.Int("board_id", boardIDInt),
		zap.Int("pon_id", ponIDInt),
	)

	// ?nocache=true forces a fresh SNMP read (bypassing the serial-list cache)
	// and refreshes it — used by write-olt-zte's pre-write existence checks so a
	// delete/replace right after a provision isn't rejected by a stale cache.
	ctx := r.Context()
	if r.URL.Query().Get("nocache") == "true" {
		ctx = usecase.WithNoCache(ctx)
	}

	// Call usecase to get Serial Number from SNMP
	onuSerialNumber, err := o.ponUsecase.GetOnuIDAndSerialNumber(ctx, boardIDInt, ponIDInt)
	if err != nil {
		log.Error("failed_to_get_onu_serial_numbers_from_snmp",
			zap.Error(err),
			zap.Int("board_id", boardIDInt),
			zap.Int("pon_id", ponIDInt),
		)
		utils.HandleError(w, r, err)
		return
	}

	log.Info("successfully_retrieved_onu_serial_numbers",
		zap.Int("board_id", boardIDInt),
		zap.Int("pon_id", ponIDInt),
		zap.Int("result_count", len(onuSerialNumber)),
	)

	// Create a web response
	response := utils.WebResponse{
		Code:   http.StatusOK,
		Status: "success",
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

	log := logger.WithRequestID(r.Context())

	log.Info("updating_empty_onu_ids",
		zap.Int("board_id", boardIDInt),
		zap.Int("pon_id", ponIDInt),
	)

	// Call usecase to get data from SNMP
	err := o.ponUsecase.UpdateEmptyOnuID(r.Context(), boardIDInt, ponIDInt)
	if err != nil {
		log.Error("failed_to_update_empty_onu_ids",
			zap.Error(err),
			zap.Int("board_id", boardIDInt),
			zap.Int("pon_id", ponIDInt),
		)
		utils.HandleError(w, r, err)
		return
	}

	log.Info("successfully_updated_empty_onu_ids",
		zap.Int("board_id", boardIDInt),
		zap.Int("pon_id", ponIDInt),
	)

	// Create a web response
	response := utils.WebResponse{
		Code:   http.StatusOK,
		Status: "success",
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

	log := logger.WithRequestID(r.Context())

	log.Info("getting_paginated_onu_info",
		zap.Int("board_id", boardIDInt),
		zap.Int("pon_id", ponIDInt),
		zap.Int("page", pageIndex),
		zap.Int("page_size", pageSize),
	)

	// Call usecase to get paginated data
	item, count := o.ponUsecase.GetByBoardIDAndPonIDWithPagination(r.Context(), boardIDInt, ponIDInt, pageIndex, pageSize)

	// Check if no items found
	if len(item) == 0 {
		log.Warn("no_onu_data_found_for_page",
			zap.Int("board_id", boardIDInt),
			zap.Int("pon_id", ponIDInt),
			zap.Int("page", pageIndex),
		)
		appErr := apperrors.NewNotFoundError("ONU data",
			map[string]interface{}{
				"board_id": boardIDInt,
				"pon_id":   ponIDInt,
				"page":     pageIndex,
			}) // Create not found error
		utils.HandleError(w, r, appErr) // Handle error
		return
	}

	// Convert result to JSON format according to Pages structure
	pages := pagination.New(pageIndex, pageSize, count) // Create pagination meta data

	log.Info("successfully_retrieved_paginated_onu_info",
		zap.Int("board_id", boardIDInt),
		zap.Int("pon_id", ponIDInt),
		zap.Int("page", pageIndex),
		zap.Int("page_size", pageSize),
		zap.Int("total_rows", pages.TotalRows),
		zap.Int("page_count", pages.PageCount),
	)

	// Create pagination response
	response := utils.WebResponse{
		Code:   http.StatusOK,
		Status: "success",
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

	log := logger.WithRequestID(r.Context())

	log.Info("deleting_cache_for_board_pon",
		zap.Int("board_id", boardIDInt),
		zap.Int("pon_id", ponIDInt),
	)

	// Call usecase to delete cache
	err := o.ponUsecase.DeleteCache(r.Context(), boardIDInt, ponIDInt)
	if err != nil {
		log.Error("failed_to_delete_cache",
			zap.Error(err),
			zap.Int("board_id", boardIDInt),
			zap.Int("pon_id", ponIDInt),
		)
		utils.HandleError(w, r, err)
		return
	}

	log.Info("successfully_deleted_cache",
		zap.Int("board_id", boardIDInt),
		zap.Int("pon_id", ponIDInt),
	)

	// Send success response
	utils.SendJSONResponse(w, http.StatusOK, utils.WebResponse{
		Code:   http.StatusOK,
		Status: "success",
		Data: map[string]interface{}{
			"message":  "Cache deleted successfully",
			"board_id": boardIDInt,
			"pon_id":   ponIDInt,
		},
	})
}

// GetUplinkTopology auto-detects the OLT's cards and uplink ethernet ports via
// SNMP (read-only). It is not board/pon-scoped — it walks standard MIBs.
// example: GET /api/v1/uplinks  or  GET /api/v1/olt/{id}/uplinks
func (o *OnuHandler) GetUplinkTopology(w http.ResponseWriter, r *http.Request) {
	log := logger.WithRequestID(r.Context())
	log.Info("getting_uplink_topology")

	topo, err := o.ponUsecase.GetUplinkTopology(r.Context())
	if err != nil {
		log.Error("failed_to_get_uplink_topology", zap.Error(err))
		utils.HandleError(w, r, err)
		return
	}

	log.Info("successfully_retrieved_uplink_topology",
		zap.Int("card_count", len(topo.Cards)),
		zap.Int("uplink_port_count", len(topo.Ports)),
	)

	utils.SendJSONResponse(w, http.StatusOK, utils.WebResponse{
		Code:   http.StatusOK,
		Status: "success",
		Data:   topo,
	})
}

// InvalidateOnuCache clears the cached board/PON list AND the per-ONU detail so
// the next read re-queries SNMP. Used after a write-olt-zte delete/replace/restart
// (which snmp-olt-zte's cache can't see) so the ONU Browser shows fresh state.
// example: DELETE /api/v1/olt/{id}/board/{board}/pon/{pon}/onu/{onu}/cache/clear
func (o *OnuHandler) InvalidateOnuCache(w http.ResponseWriter, r *http.Request) {
	boardIDInt, _ := middleware.GetBoardID(r.Context())
	ponIDInt, _ := middleware.GetPonID(r.Context())
	onuIDInt, _ := middleware.GetOnuID(r.Context())

	log := logger.WithRequestID(r.Context())
	log.Info("invalidating_onu_cache",
		zap.Int("board_id", boardIDInt),
		zap.Int("pon_id", ponIDInt),
		zap.Int("onu_id", onuIDInt),
	)

	if err := o.ponUsecase.InvalidateONUCache(r.Context(), boardIDInt, ponIDInt, onuIDInt); err != nil {
		log.Error("failed_to_invalidate_onu_cache",
			zap.Error(err),
			zap.Int("board_id", boardIDInt),
			zap.Int("pon_id", ponIDInt),
			zap.Int("onu_id", onuIDInt),
		)
		utils.HandleError(w, r, err)
		return
	}

	utils.SendJSONResponse(w, http.StatusOK, utils.WebResponse{
		Code:   http.StatusOK,
		Status: "success",
		Data: map[string]interface{}{
			"message":  "ONU cache invalidated",
			"board_id": boardIDInt,
			"pon_id":   ponIDInt,
			"onu_id":   onuIDInt,
		},
	})
}
