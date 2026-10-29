package instancesettings

import (
	"context"
	"math/rand/v2"
	"time"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
)

func (s *Store) Run(ctx context.Context) {
	if s.listen == nil {
		return
	}
	backoff := config.JobListenRetryMin
	for ctx.Err() == nil {
		err := s.serve(ctx)
		if ctx.Err() != nil {
			return
		}
		s.log.WarnContext(ctx, "instance settings listener stopped, retrying", "error", err, "retry_in", backoff.String())
		timer := time.NewTimer(backoff/2 + rand.N(backoff/2+1)) //nolint:gosec // Retry jitter needs no cryptographic randomness.
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		backoff = min(backoff*2, config.JobListenRetryMax)
	}
}

func (s *Store) serve(ctx context.Context) error {
	conn, err := s.listen(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := conn.Close(context.WithoutCancel(ctx)); cerr != nil {
			s.log.WarnContext(ctx, "close instance settings listener", "error", cerr)
		}
	}()
	if err := sqlc.New(conn).ListenSettingsChanged(ctx); err != nil {
		return err
	}
	s.Reload(ctx)
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		if n.Channel != ChangedChannel {
			continue
		}
		s.Reload(ctx)
		s.log.InfoContext(ctx, "instance settings reloaded")
	}
}
