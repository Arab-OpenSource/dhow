package app

import (
	"context"
	"sync"
	"time"

	session "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session"
)

func StartMaintenance(ctx context.Context, s *session.Service, every time.Duration) func() {
	bgCtx, cancel := context.WithCancel(ctx)
	var background sync.WaitGroup
	background.Go(func() { s.RunOidcFlowPurge(bgCtx, every) })
	return func() {
		cancel()
		background.Wait()
	}
}
