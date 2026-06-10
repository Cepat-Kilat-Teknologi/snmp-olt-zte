package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	apperrors "github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/errors"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/utils"
	"github.com/go-chi/chi/v5"
)

// ContextKey is the type for context keys to avoid collisions
type ContextKey string

const (
	// BoardIDKey is the context key for board ID
	BoardIDKey ContextKey = "boardID"
	// PonIDKey is the context key for PON ID
	PonIDKey ContextKey = "ponID"
	// OnuIDKey is the context key for ONU ID
	OnuIDKey ContextKey = "onuID"
)

// GetBoardID retrieves the validated board ID from request context
func GetBoardID(ctx context.Context) (int, bool) {
	val := ctx.Value(BoardIDKey)
	if val == nil {
		return 0, false
	}
	boardID, ok := val.(int)
	return boardID, ok
}

// GetPonID retrieves the validated PON ID from request context
func GetPonID(ctx context.Context) (int, bool) {
	val := ctx.Value(PonIDKey)
	if val == nil {
		return 0, false
	}
	ponID, ok := val.(int)
	return ponID, ok
}

// GetOnuID retrieves the validated ONU ID from request context
func GetOnuID(ctx context.Context) (int, bool) {
	val := ctx.Value(OnuIDKey)
	if val == nil {
		return 0, false
	}
	onuID, ok := val.(int)
	return onuID, ok
}

// ValidateBoardPonParams returns middleware that validates the board_id and
// pon_id URL parameters against a PER-SLOT PON topology. boardPons maps each
// configured physical GPON slot to its card's PON-port count (GTGO=8, GTGH=16),
// so pon_id is validated against THAT slot's card — /board/5/pon/16 is rejected
// when slot 5 is an 8-port GTGO. A board_id not in boardPons, or a pon_id
// outside [1, that slot's count], yields a 400. An empty map falls back to the
// legacy C320 defaults ({1:16, 2:16}).
func ValidateBoardPonParams(boardPons map[int]int) func(http.Handler) http.Handler {
	if len(boardPons) == 0 {
		boardPons = map[int]int{1: 16, 2: 16}
	}

	// Pre-render the allowed-slots list for error payloads (sorted for stability).
	allowed := make([]int, 0, len(boardPons))
	for b := range boardPons {
		allowed = append(allowed, b)
	}
	sort.Ints(allowed)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			boardID := chi.URLParam(r, "board_id") // Get board_id from URL
			ponID := chi.URLParam(r, "pon_id")     // Get pon_id from URL

			// Validate board_id: must be an integer and a configured GPON slot.
			boardIDInt, err := strconv.Atoi(boardID)
			maxPon, ok := boardPons[boardIDInt]
			if err != nil || !ok {
				appErr := apperrors.NewValidationError(
					"board_id is not a configured GPON slot",
					map[string]interface{}{"received": boardID, "allowed": allowed},
				)
				utils.HandleError(w, r, appErr)
				return
			}

			// Validate pon_id: must be within [1, this slot's PON count].
			ponIDInt, err := strconv.Atoi(ponID)
			if err != nil || ponIDInt < 1 || ponIDInt > maxPon {
				appErr := apperrors.NewValidationError(
					fmt.Sprintf("pon_id must be between 1 and %d for board %d", maxPon, boardIDInt),
					map[string]interface{}{"received": ponID},
				)
				utils.HandleError(w, r, appErr)
				return
			}

			// Store validated values into request context for easier access in handlers
			ctx := r.Context()
			ctx = context.WithValue(ctx, BoardIDKey, boardIDInt)
			ctx = context.WithValue(ctx, PonIDKey, ponIDInt)

			next.ServeHTTP(w, r.WithContext(ctx)) // Proceed with the updated context
		})
	}
}

// ValidateOnuIDParam validates onu_id URL parameter,
// ensuring it is a valid integer within the expected range (1-128).
func ValidateOnuIDParam(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		onuID := chi.URLParam(r, "onu_id") // Get onu_id from URL

		// Validate onu_id conversion to integer
		onuIDInt, err := strconv.Atoi(onuID)
		// Check if conversion failed or if onu_id is out of range (1-128)
		if err != nil || onuIDInt < 1 || onuIDInt > 128 {
			appErr := apperrors.NewValidationError(
				"onu_id must be between 1 and 128",
				map[string]interface{}{"received": onuID},
			) // Create validation error
			utils.HandleError(w, r, appErr) // Return error response
			return
		}

		// Store validated value into context
		ctx := context.WithValue(r.Context(), OnuIDKey, onuIDInt)
		next.ServeHTTP(w, r.WithContext(ctx)) // Proceed with the updated context
	})
}
