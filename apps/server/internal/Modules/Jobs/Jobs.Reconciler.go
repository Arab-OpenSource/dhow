package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Tx"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Jobs/Models"
)

type ReconcileResult struct {
	ExpiredLeases int
	RevokedLeases int
	TimedOut      int
	QueueTimeouts int
}

func (r ReconcileResult) Total() int {
	return r.ExpiredLeases + r.RevokedLeases + r.TimedOut + r.QueueTimeouts
}

type picker func(ctx context.Context, q *sqlc.Queries) (uuid.UUID, error)

func lost(reason, leaseState string) models.Outcome {
	return models.Outcome{State: models.AttemptLost, FailureReason: reason, LeaseState: leaseState}
}

func (s *Service) Reconcile(ctx context.Context) (ReconcileResult, error) {
	var out ReconcileResult
	var err error
	grace := config.JobLeaseGrace.Seconds()
	if out.ExpiredLeases, err = s.sweep(ctx, func(ctx context.Context, q *sqlc.Queries) (uuid.UUID, error) {
		return q.PickExpiredLease(ctx, grace)
	}, lost(models.ReasonLeaseLost, models.LeaseExpired)); err != nil {
		return out, fmt.Errorf("expire leases: %w", err)
	}
	if out.RevokedLeases, err = s.sweep(ctx, func(ctx context.Context, q *sqlc.Queries) (uuid.UUID, error) {
		return q.PickRevokedRunnerLease(ctx)
	}, lost(models.ReasonRunnerRevoked, models.LeaseRevoked)); err != nil {
		return out, fmt.Errorf("revoke leases of revoked runners: %w", err)
	}
	deadline := config.JobDeadlineGrace.Seconds()
	if out.TimedOut, err = s.sweep(ctx, func(ctx context.Context, q *sqlc.Queries) (uuid.UUID, error) {
		return q.PickOverdueLease(ctx, deadline)
	}, models.Outcome{State: models.AttemptTimedOut, FailureReason: models.ReasonTimedOut, LeaseState: models.LeaseRevoked}); err != nil {
		return out, fmt.Errorf("time out jobs: %w", err)
	}
	rows, err := s.queries().FailTimedOutQueuedJobs(ctx, config.JobReconcileBatch)
	if err != nil {
		return out, fmt.Errorf("fail jobs past their queue deadline: %w", err)
	}
	out.QueueTimeouts = len(rows)
	return out, nil
}

func (s *Service) sweep(ctx context.Context, pick picker, o models.Outcome) (int, error) {
	done := 0
	for done < config.JobReconcileBatch {
		found := false
		err := dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
			id, err := pick(ctx, q)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			found = true
			return s.finalize(ctx, q, id, o)
		})
		if err != nil {
			return done, err
		}
		if !found {
			return done, nil
		}
		done++
	}
	return done, nil
}

func (s *Service) RunReconciler(ctx context.Context) {
	ticker := time.NewTicker(config.JobReconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		res, err := s.Reconcile(ctx)
		switch {
		case err != nil && ctx.Err() == nil:
			s.log.ErrorContext(ctx, "job reconciler pass failed", "error", err)
		case res.Total() > 0:
			s.log.InfoContext(ctx, "job reconciler pass", "expired_leases", res.ExpiredLeases, "revoked_leases", res.RevokedLeases,
				"timed_out", res.TimedOut, "queue_timeouts", res.QueueTimeouts)
		}
	}
}

func (s *Service) RunListener(ctx context.Context) { s.hub.run(ctx) }

func (s *Service) HandleRunnerRevoked(ctx context.Context, ev eventbus.Event) error {
	a, ok := ev.Payload.(eventbus.Audited)
	if !ok {
		return fmt.Errorf("event %s carries an unexpected payload", ev.Name)
	}
	runner, err := uuid.Parse(a.TargetID)
	if err != nil {
		return fmt.Errorf("event %s names no runner: %w", ev.Name, err)
	}
	n, err := s.RevokeRunnerLeases(ctx, runner)
	if err != nil {
		s.log.ErrorContext(ctx, "end the leases of a revoked runner; the reconciler retries", "runner_id", runner.String(), "error", err)
		return nil
	}
	if n > 0 {
		s.log.InfoContext(ctx, "ended the leases of a revoked runner", "runner_id", runner.String(), "leases", n)
	}
	return nil
}

func (s *Service) RevokeRunnerLeases(ctx context.Context, runner uuid.UUID) (int, error) {
	return s.sweep(ctx, func(ctx context.Context, q *sqlc.Queries) (uuid.UUID, error) {
		if err := q.LockRunnerSlots(ctx, runner.String()); err != nil {
			return uuid.Nil, err
		}
		return q.LockRunnerLease(ctx, runner)
	}, lost(models.ReasonRunnerRevoked, models.LeaseRevoked))
}
