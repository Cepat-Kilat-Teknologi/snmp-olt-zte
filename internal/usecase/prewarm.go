package usecase

import (
	"context"
	"time"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	"go.uber.org/zap"
)

// PreWarmCache scans all board/pon combinations to populate Redis cache.
// This runs as a background goroutine at startup so first requests hit cache.
func (u *onuUsecase) PreWarmCache(ctx context.Context) {
	logger.Info("cache_prewarm_starting")
	start := time.Now()
	total := 0
	errors := 0

	// Iterate over the configured (slot, pon) set rather than a hardcoded 2x16
	// grid, so C300 layouts (e.g. slots 3,5) are pre-warmed correctly. The
	// BoardPonMap is the single source of truth for which slots/PONs exist.
	for key := range u.cfg.BoardPonMap {
		select {
		case <-ctx.Done():
			logger.Warn("cache_prewarm_canceled")
			return
		default:
		}

		_, err := u.GetByBoardIDAndPonID(ctx, key.BoardID, key.PonID)
		if err != nil {
			errors++
			logger.Debug("cache_prewarm_fetch_onu_failed",
				zap.Error(err),
				zap.Int("board", key.BoardID),
				zap.Int("pon", key.PonID),
			)
		} else {
			total++
		}

		// Also pre-warm serial number list cache.
		_, err = u.GetOnuIDAndSerialNumber(ctx, key.BoardID, key.PonID)
		if err != nil {
			logger.Debug("cache_prewarm_fetch_sn_failed",
				zap.Error(err),
				zap.Int("board", key.BoardID),
				zap.Int("pon", key.PonID),
			)
		}
	}

	logger.Info("cache_prewarm_completed",
		zap.Int("success", total),
		zap.Int("errors", errors),
		zap.Int64("duration_ms", time.Since(start).Milliseconds()),
	)
}
