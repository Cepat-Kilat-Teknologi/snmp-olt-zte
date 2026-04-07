package usecase

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// PreWarmCache scans all board/pon combinations to populate Redis cache.
// This runs as a background goroutine at startup so first requests hit cache.
func (u *onuUsecase) PreWarmCache(ctx context.Context) {
	log.Info().Msg("Cache pre-warm: starting")
	start := time.Now()
	total := 0
	errors := 0

	for boardID := 1; boardID <= 2; boardID++ {
		for ponID := 1; ponID <= 16; ponID++ {
			select {
			case <-ctx.Done():
				log.Warn().Msg("Cache pre-warm: cancelled")
				return
			default:
			}

			_, err := u.GetByBoardIDAndPonID(ctx, boardID, ponID)
			if err != nil {
				errors++
				log.Debug().Err(err).Int("board", boardID).Int("pon", ponID).
					Msg("Cache pre-warm: failed to fetch ONU info")
			} else {
				total++
			}

			// Also pre-warm serial number list cache
			_, err = u.GetOnuIDAndSerialNumber(ctx, boardID, ponID)
			if err != nil {
				log.Debug().Err(err).Int("board", boardID).Int("pon", ponID).
					Msg("Cache pre-warm: failed to fetch serial numbers")
			}
		}
	}

	log.Info().
		Int("success", total).
		Int("errors", errors).
		Dur("duration", time.Since(start)).
		Msg("Cache pre-warm: completed")
}
