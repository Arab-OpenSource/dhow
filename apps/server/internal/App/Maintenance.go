package app

import (
	"context"
	"sync"
	"time"

	retention "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Retention"
)

func StartMaintenance(ctx context.Context, r *retention.Runner, every time.Duration) func() {
	bgCtx, cancel := context.WithCancel(ctx)
	var background sync.WaitGroup
	background.Go(func() { r.Loop(bgCtx, every) })
	return func() {
		cancel()
		background.Wait()
	}
}
