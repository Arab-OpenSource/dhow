package session

import (
	"context"
	"time"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
)

func (s *Service) PurgeOidcFlows(ctx context.Context) (int64, error) {
	return s.queries().PurgeOidcFlows(ctx, s.now())
}

func (s *Service) RunOidcFlowPurge(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = config.OidcFlowPurgeEvery
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		n, err := s.PurgeOidcFlows(ctx)
		switch {
		case err != nil && ctx.Err() == nil:
			s.log.ErrorContext(ctx, "oidc flow purge failed", "error", err)
		case n > 0:
			s.log.InfoContext(ctx, "oidc flows purged", "count", n)
		}
	}
}
