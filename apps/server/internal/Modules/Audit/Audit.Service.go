package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Tx"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Audit/Models"
	"github.com/Arab-OpenSource/dhow/internal/logging"
)

const (
	walkBatch        = 500
	maxDataEntries   = 24
	viaKey           = "Via"
	viaIDKey         = "ViaId"
	viaAPIToken      = "api_token"
	maxDataValueLen  = 256
	instanceChainKey = "instance"
)

type DB interface {
	sqlc.DBTX
	dbtx.Beginner
}

type Service struct {
	db  DB
	log *slog.Logger
	now func() time.Time
}

func NewService(db DB, log *slog.Logger, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{db: db, log: log, now: now}
}

func (s *Service) Handle(ctx context.Context, ev eventbus.Event) error {
	a, ok := ev.Payload.(eventbus.Audited)
	if !ok {
		return fmt.Errorf("audit event %s carries an unexpected payload", ev.Name)
	}
	chains := make([]*uuid.UUID, 0, max(len(a.OrganizationIDs), 1))
	ids := append([]uuid.UUID(nil), a.OrganizationIDs...)
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
	for i := range ids {
		chains = append(chains, &ids[i])
	}
	if len(chains) == 0 {
		chains = append(chains, nil)
	}
	for _, org := range chains {
		if err := s.append(ctx, org, ev.Name, a); err != nil {
			return fmt.Errorf("record audit event %s: %w", ev.Name, err)
		}
	}
	return nil
}

func withCredential(ctx context.Context, data map[string]string) map[string]string {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok || !p.ViaToken() {
		return data
	}
	data[viaKey], data[viaIDKey] = viaAPIToken, p.TokenID.String()
	return data
}

func chainKey(org *uuid.UUID) string {
	if org == nil {
		return instanceChainKey
	}
	return "org:" + org.String()
}

func (s *Service) append(ctx context.Context, org *uuid.UUID, action string, a eventbus.Audited) error {
	return dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if err := q.LockAuditChain(ctx, chainKey(org)); err != nil {
			return err
		}
		prev, err := q.GetLastAuditHash(ctx, org)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			prev = models.GenesisHash()
		case err != nil:
			return err
		}
		id, err := q.NextAuditId(ctx)
		if err != nil {
			return err
		}
		ev := models.Event{
			ID:             id,
			CreatedAt:      s.now().UTC().Truncate(time.Microsecond),
			OrganizationID: org,
			ActorUserID:    a.ActorUserID,
			Action:         action,
			TargetType:     a.TargetType,
			TargetID:       a.TargetID,
			IP:             a.IP,
			Data:           withCredential(ctx, sanitizeData(a.Data)),
			PrevHash:       prev,
		}
		if ev.Hash, err = models.ComputeHash(prev, ev); err != nil {
			return err
		}
		data, err := json.Marshal(ev.Data)
		if err != nil {
			return fmt.Errorf("encode audit data: %w", err)
		}
		return q.InsertAuditEvent(ctx, sqlc.InsertAuditEventParams{
			ID: ev.ID, CreatedAt: ev.CreatedAt, OrganizationID: org, ActorUserID: ev.ActorUserID, Action: ev.Action,
			TargetType: ev.TargetType, TargetID: ev.TargetID, Ip: ev.IP, Data: data, PrevHash: prev, Hash: ev.Hash,
		})
	})
}

func sanitizeData(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if len(out) >= maxDataEntries {
			break
		}
		if logging.IsSensitiveKey(k) || k == viaKey || k == viaIDKey {
			continue
		}
		v := in[k]
		if len(v) > maxDataValueLen {
			v = v[:maxDataValueLen]
		}
		out[k] = v
	}
	return out
}

func (s *Service) List(ctx context.Context, org uuid.UUID, page, limit int) ([]models.Event, int64, error) {
	q := sqlc.New(s.db)
	total, err := q.CountAuditEvents(ctx, &org)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.ListAuditEvents(ctx, sqlc.ListAuditEventsParams{
		OrganizationID: &org, PageLimit: int32(limit), PageOffset: int32((page - 1) * limit), //nolint:gosec // limit and page are bounded by response.PageParams.
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]models.Event, 0, len(rows))
	for _, r := range rows {
		data := map[string]string{}
		if err := json.Unmarshal(r.Data, &data); err != nil {
			return nil, 0, fmt.Errorf("decode audit data: %w", err)
		}
		out = append(out, models.Event{
			ID: r.Id, CreatedAt: r.CreatedAt, OrganizationID: r.OrganizationId, ActorUserID: r.ActorUserId, Action: r.Action,
			TargetType: r.TargetType, TargetID: r.TargetId, IP: r.Ip, Data: data,
		})
	}
	return out, total, nil
}

func (s *Service) Verify(ctx context.Context, org uuid.UUID) (models.VerifyResult, error) {
	q := sqlc.New(s.db)
	result := models.VerifyResult{Valid: true}
	var after int64
	prev := models.GenesisHash()
	for {
		rows, err := q.WalkAuditChain(ctx, sqlc.WalkAuditChainParams{OrganizationID: &org, AfterSeq: after, PageLimit: walkBatch})
		if err != nil {
			return models.VerifyResult{}, err
		}
		for _, r := range rows {
			data := map[string]string{}
			if err := json.Unmarshal(r.Data, &data); err != nil {
				return models.VerifyResult{}, fmt.Errorf("decode audit data: %w", err)
			}
			ev := models.Event{
				ID: r.Id, CreatedAt: r.CreatedAt, OrganizationID: r.OrganizationId, ActorUserID: r.ActorUserId, Action: r.Action,
				TargetType: r.TargetType, TargetID: r.TargetId, IP: r.Ip, Data: data, PrevHash: r.PrevHash, Hash: r.Hash,
			}
			want, err := models.ComputeHash(r.PrevHash, ev)
			if err != nil {
				return models.VerifyResult{}, err
			}
			linked := bytes.Equal(prev, r.PrevHash)
			if !linked || !bytes.Equal(want, r.Hash) {
				id := r.Id
				return models.VerifyResult{Valid: false, Checked: result.Checked + 1, FirstBrokenID: &id}, nil
			}
			result.Checked++
			prev, after = r.Hash, r.Seq
		}
		if len(rows) < walkBatch {
			return result, nil
		}
	}
}
