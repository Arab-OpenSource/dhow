package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	instancesettings "github.com/Khaymah/dhow/apps/server/internal/Core/Settings"
	jobs "github.com/Khaymah/dhow/apps/server/internal/Modules/Jobs"
)

func StartSettings(ctx context.Context, store *instancesettings.Store) func() {
	bgCtx, cancel := context.WithCancel(ctx)
	var background sync.WaitGroup
	background.Go(func() { store.Run(bgCtx) })
	return func() {
		cancel()
		background.Wait()
	}
}

func StartJobs(ctx context.Context, j *jobs.Service, log *slog.Logger) (func(), error) {
	parts, err := j.MaintainLogPartitions(ctx)
	if err != nil {
		return nil, fmt.Errorf("prepare job log partitions before serving: %w", err)
	}
	log.Info("job log partitions ready", "created", parts.Created, "dropped", parts.Dropped, "skipped", parts.Skipped)
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
	background.Go(func() { j.RunLogMaintenance(bgCtx) })
	return func() {
		cancel()
		background.Wait()
	}, nil
}
