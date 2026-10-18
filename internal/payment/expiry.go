package payment

import (
	"context"
	"log/slog"
	"time"
)

// Retry transient DB failures on the next bounded scan. Expiry and reviews use
// the same request row locks; a pending timely complete set is never expired.
func RunExpiry(ctx context.Context, repo ExpiryRepository, logger *slog.Logger) error {
	timer := time.NewTicker(15 * time.Second)
	defer timer.Stop()
	for {
		if ctx.Err() != nil {
			return nil
		}
		scan, cancel := context.WithTimeout(ctx, 10*time.Second)
		n, err := repo.ExpireUnfulfilled(scan)
		cancel()
		if err != nil && ctx.Err() == nil {
			logger.Warn("UC4 expiry scan failed; will retry")
		}
		if n > 0 {
			logger.Info("UC4 requests expired", "count", n)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
	}
}
