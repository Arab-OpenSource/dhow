package tokens

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
)

func (s *Service) ResolveToken(ctx context.Context, hash []byte, now time.Time) (auth.ResolvedToken, error) {
	q := s.queries()
	row, err := q.ResolveApiToken(ctx, sqlc.ResolveApiTokenParams{TokenHash: hash, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ResolvedToken{}, auth.ErrNoToken
	}
	if err != nil {
		return auth.ResolvedToken{}, err
	}
	role, ok := policy.EffectiveOf(row.Role, row.TeamRoles)
	if !ok {
		return auth.ResolvedToken{}, auth.ErrNoToken
	}
	scopes := make([]policy.Action, 0, len(row.Scopes))
	for _, raw := range row.Scopes {
		if a := policy.Action(raw); policy.IsTokenAction(a) {
			scopes = append(scopes, a)
		}
	}
	if row.LastUsedAt == nil || now.Sub(*row.LastUsedAt) >= config.APITokenTouchInterval {
		touchErr := q.TouchApiToken(ctx, sqlc.TouchApiTokenParams{
			ID: row.Id, Now: now, Ip: auth.RequestMetaFrom(ctx).IPString(), TouchBefore: now.Add(-config.APITokenTouchInterval),
		})
		if touchErr != nil {
			s.log.WarnContext(ctx, "touch api token failed", "error", touchErr)
		}
	}
	rule, known := policy.ParseSecondFactorRule(row.RequireSecondFactor)
	if !known {
		rule = policy.SecondFactorEveryone
	}
	state := policy.SecondFactorState{
		Rule: rule, Since: row.RequireSecondFactorSince, AdminsSince: row.RequireSecondFactorAdminsSince, JoinedAt: row.JoinedAt, Role: role, HasFactor: row.HasSecondFactor,
	}
	due := policy.SecondFactorDue(state, config.SecondFactorGrace)
	return auth.ResolvedToken{
		TokenID: row.Id, UserID: row.UserId, OrganizationID: row.OrganizationId, Role: role, Scopes: scopes,
		ExpiresAt: row.ExpiresAt, SecondFactorDue: due, AdminDue: policy.SecondFactorDueAsAdmin(state, config.SecondFactorGrace), Email: row.Email, Name: row.Name, Locale: row.Locale,
	}, nil
}
