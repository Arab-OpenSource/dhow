package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	jobs "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Jobs"
)

func StartJobs(ctx context.Context, j *jobs.Service, log *slog.Logger) (func(), error) {
	res, err := j.Reconcile(ctx)
	if err != nil {
		return nil, fmt.Errorf("reconcile jobs before serving: %w", err)
	}
	log.Info("jobs reconciled", "expired_leases", res.ExpiredLeases, "revoked_leases", res.RevokedLeases,
		"canceled_unstarted", res.CanceledUnstarted, "start_timeouts", res.StartTimeouts, "timed_out", res.TimedOut,
		"queue_timeouts", res.QueueTimeouts)
	bgCtx, cancel := context.WithCancel(ctx)
	var background sync.WaitGroup
	background.Go(func() { j.RunListener(bgCtx) })
	background.Go(func() { j.RunReconciler(bgCtx) })
	return func() {
		cancel()
		background.Wait()
	}, nil
}
