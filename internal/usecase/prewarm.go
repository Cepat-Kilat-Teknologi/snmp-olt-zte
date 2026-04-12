package usecase

import (
	"context"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/logger"
	"go.uber.org/zap"
)

// PreWarmCache scans all board/pon combinations to populate Redis cache.
// This runs as a background goroutine at startup so first requests hit cache.
func (u *onuUsecase) PreWarmCache(ctx context.Context) {
	logger.Info("cache_prewarm_starting")
	start := time.Now()
	total := 0
	errors := 0

	for boardID := 1; boardID <= 2; boardID++ {
		for ponID := 1; ponID <= 16; ponID++ {
			select {
			case <-ctx.Done():
				logger.Warn("cache_prewarm_cancelled")
				return
			default:
			}

			_, err := u.GetByBoardIDAndPonID(ctx, boardID, ponID)
			if err != nil {
				errors++
				logger.Debug("cache_prewarm_fetch_onu_failed",
					zap.Error(err),
					zap.Int("board", boardID),
					zap.Int("pon", ponID),
				)
			} else {
				total++
			}

			// Also pre-warm serial number list cache.
			_, err = u.GetOnuIDAndSerialNumber(ctx, boardID, ponID)
			if err != nil {
				logger.Debug("cache_prewarm_fetch_sn_failed",
					zap.Error(err),
					zap.Int("board", boardID),
					zap.Int("pon", ponID),
				)
			}
		}
	}

	logger.Info("cache_prewarm_completed",
		zap.Int("success", total),
		zap.Int("errors", errors),
		zap.Int64("duration_ms", time.Since(start).Milliseconds()),
	)
}
