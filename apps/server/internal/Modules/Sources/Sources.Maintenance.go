package sources

import (
	"context"
	"errors"
	"fmt"
	"time"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Khaymah/dhow/apps/server/internal/Database/Tx"
)

var ErrDeliveryPartitionMissing = errors.New("no webhook delivery partition covers the current day")

type MaintenanceResult struct {
	Skipped     bool
	Created     int
	Dropped     int
	ExpiredKeys int64
}

func (s *Service) Maintain(ctx context.Context) (MaintenanceResult, error) {
	var out MaintenanceResult
	err := dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		locked, err := q.TryLockWebhookPartitions(ctx)
		if err != nil {
			return err
		}
		if !locked {
			out.Skipped = true
			return nil
		}
		if err := q.SetLocalLockTimeout(ctx, config.WebhookMaintainLockWait.String()); err != nil {
			return err
		}
		created, err := q.EnsureWebhookPartitions(ctx, config.WebhookPartitionsAhead)
		if err != nil {
			return fmt.Errorf("create webhook delivery partitions: %w", err)
		}
		dropped, err := q.DropWebhookPartitions(ctx, config.WebhookRetentionDays)
		if err != nil {
			return fmt.Errorf("drop expired webhook delivery partitions: %w", err)
		}
		out.Created, out.Dropped = int(created), int(dropped)
		out.ExpiredKeys, err = q.ExpirePreviousDeployKeys(ctx, s.now())
		return err
	})
	return out, err
}

func (s *Service) RunMaintenance(ctx context.Context) {
	ticker := time.NewTicker(config.WebhookMaintainInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		res, err := s.Maintain(ctx)
		switch {
		case err != nil && ctx.Err() == nil:
			s.log.ErrorContext(ctx, "webhook delivery maintenance failed; the next pass retries", "error", err)
		case res.Created > 0 || res.Dropped > 0 || res.ExpiredKeys > 0:
			s.log.InfoContext(ctx, "webhook deliveries maintained", "created", res.Created, "dropped", res.Dropped,
				"expired_deploy_keys", res.ExpiredKeys)
		}
	}
}

func (s *Service) CheckDeliveryPartition(ctx context.Context) error {
	ok, err := s.queries().WebhookPartitionReady(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDeliveryPartitionMissing
	}
	return nil
}
