package identity

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	ratelimit "github.com/Khaymah/dhow/apps/server/internal/Core/RateLimit"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Khaymah/dhow/apps/server/internal/Database/Tx"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Identity/Models"
)

const (
	constraintPendingInvite = "UX_Invitations_Organization_Email_Pending"
	limitInvitesUser        = "invite-user"
	limitInvitesOrg         = "invite-org"
)

type inviteToken struct {
	text string
	hash []byte
}

func newInviteToken() (inviteToken, error) {
	raw := make([]byte, config.InvitationTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return inviteToken{}, fmt.Errorf("read random bytes: %w", err)
	}
	return inviteToken{text: base64.RawURLEncoding.EncodeToString(raw), hash: auth.HashBytes(raw)}, nil
}

func decodeInviteToken(v string) ([]byte, bool) {
	return auth.HashSessionToken(v, config.InvitationTokenBytes)
}

func (s *Service) inviteLink(t inviteToken) string { return s.publicURL + "/invite#" + t.text }

func (s *Service) initialEmailStatus() string {
	if s.mailer.Enabled() {
		return models.EmailFailed
	}
	return models.EmailDisabled
}

func invitationOf(r sqlc.GetOpenInvitationRow, now time.Time) models.Invitation {
	return models.Invitation{
		ID: r.Id, Email: r.Email, Role: r.Role, InvitedByName: r.InvitedByName, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt,
		SentAt: r.SentAt, SendCount: int(r.SendCount), EmailStatus: r.EmailStatus, Expired: !r.ExpiresAt.After(now),
	}
}

func (s *Service) takeInviteLimits(ctx context.Context, org, user uuid.UUID) error {
	limits := s.limits()
	checks := []struct {
		limit   ratelimit.Limit
		subject uuid.UUID
	}{
		{ratelimit.Limit{Name: limitInvitesUser, Max: limits.InvitesUser, Window: config.RateLimitInviteWindow}, user},
		{ratelimit.Limit{Name: limitInvitesOrg, Max: limits.InvitesOrg, Window: config.RateLimitInviteWindow}, org},
	}
	for _, c := range checks {
		d, err := s.limiter.Take(ctx, c.limit, c.subject.String())
		if err != nil {
			return err
		}
		if err := d.Err(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListInvitations(ctx context.Context, org uuid.UUID, status string) ([]models.Invitation, error) {
	if status == models.InvitationsAccepted {
		return s.listAcceptedInvitations(ctx, org)
	}
	rows, err := s.queries().ListOpenInvitations(ctx, sqlc.ListOpenInvitationsParams{OrganizationID: org, MaxRows: config.InvitationListLimit})
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := make([]models.Invitation, 0, len(rows))
	for _, r := range rows {
		out = append(out, invitationOf(sqlc.GetOpenInvitationRow(r), now))
	}
	return out, nil
}

func (s *Service) listAcceptedInvitations(ctx context.Context, org uuid.UUID) ([]models.Invitation, error) {
	rows, err := s.queries().ListAcceptedInvitations(ctx, sqlc.ListAcceptedInvitationsParams{OrganizationID: org, MaxRows: config.InvitationListLimit})
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := make([]models.Invitation, 0, len(rows))
	for _, r := range rows {
		i := invitationOf(sqlc.GetOpenInvitationRow{
			Id: r.Id, Email: r.Email, Role: r.Role, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, SentAt: r.SentAt,
			SendCount: r.SendCount, EmailStatus: r.EmailStatus, InvitedByName: r.InvitedByName,
		}, now)
		i.Expired = false
		i.Accepted = &models.InvitationAcceptance{At: r.AcceptedAt, UserID: r.AcceptedByUserId, UserName: r.AcceptedByName}
		out = append(out, i)
	}
	return out, nil
}

func (s *Service) CreateInvitation(ctx context.Context, org uuid.UUID, email, role string) (models.InvitationIssued, error) {
	p, actor, err := actorIn(ctx, org)
	if err != nil {
		return models.InvitationIssued{}, err
	}
	email = auth.NormalizeEmail(email)
	var details []apperrors.IErrorDetail
	if !auth.ValidEmail(email) {
		details = append(details, invalid(fieldEmail))
	}
	r, ok := policy.ParseRole(role)
	if !ok || !policy.Bindable(r) {
		details = append(details, invalid(fieldRole))
	}
	if len(details) > 0 {
		return models.InvitationIssued{}, apperrors.ValidationFailed(details...)
	}
	if !policy.CanGrant(actor, r) {
		return models.InvitationIssued{}, apperrors.RoleNotAllowed()
	}
	if err := s.takeInviteLimits(ctx, org, p.UserID); err != nil {
		return models.InvitationIssued{}, err
	}
	token, err := newInviteToken()
	if err != nil {
		return models.InvitationIssued{}, err
	}
	now := s.now()
	var id uuid.UUID
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if err := lockOrganization(ctx, q, org); err != nil {
			return err
		}
		member, err := q.IsMemberByEmail(ctx, sqlc.IsMemberByEmailParams{OrganizationID: org, Email: email})
		if err != nil {
			return err
		}
		if member {
			return apperrors.AlreadyMember()
		}
		if err := q.RetireExpiredInvitations(ctx, sqlc.RetireExpiredInvitationsParams{OrganizationID: org, Email: email, Now: now}); err != nil {
			return err
		}
		id, err = q.InsertInvitation(ctx, sqlc.InsertInvitationParams{
			OrganizationID: org, Email: email, Role: string(r), TokenHash: token.hash, InvitedByUserID: p.UserID,
			ExpiresAt: now.Add(config.InvitationTTL), Now: now, EmailStatus: s.initialEmailStatus(),
		})
		return err
	})
	if uniqueConstraint(err) == constraintPendingInvite {
		return models.InvitationIssued{}, apperrors.InvitationPending()
	}
	if err != nil {
		return models.InvitationIssued{}, err
	}
	return s.deliver(ctx, org, id, token, eventbus.IdentityInvitationCreated)
}

func (s *Service) deliver(ctx context.Context, org, id uuid.UUID, token inviteToken, event string) (models.InvitationIssued, error) {
	q := s.queries()
	row, err := q.GetOpenInvitation(ctx, sqlc.GetOpenInvitationParams{ID: id, OrganizationID: org})
	if err != nil {
		return models.InvitationIssued{}, err
	}
	status := s.sendInvitation(ctx, org, row, s.inviteLink(token))
	if status != row.EmailStatus {
		if err := q.SetInvitationEmailStatus(ctx, sqlc.SetInvitationEmailStatusParams{ID: id, EmailStatus: status, Now: s.now()}); err != nil {
			return models.InvitationIssued{}, err
		}
		row.EmailStatus = status
	}
	issued := models.InvitationIssued{Invitation: invitationOf(row, s.now()), Link: s.inviteLink(token)}
	return issued, s.audit(ctx, event, []uuid.UUID{org}, targetInvitation, id.String(), map[string]string{
		"Email": row.Email, "Role": row.Role, "EmailStatus": status, "ExpiresAt": row.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Service) lockInvitation(ctx context.Context, q *sqlc.Queries, org, id uuid.UUID, actor policy.Role) (sqlc.LockOpenInvitationRow, error) {
	row, err := q.LockOpenInvitation(ctx, sqlc.LockOpenInvitationParams{ID: id, OrganizationID: org})
	if errors.Is(err, pgx.ErrNoRows) {
		return row, apperrors.NotFound()
	}
	if err != nil {
		return row, err
	}
	if !policy.CanGrant(actor, policy.Role(row.Role)) {
		return row, apperrors.RoleNotAllowed()
	}
	return row, nil
}

func (s *Service) RevokeInvitation(ctx context.Context, org, id uuid.UUID) (models.Invitation, error) {
	_, actor, err := actorIn(ctx, org)
	if err != nil {
		return models.Invitation{}, err
	}
	now := s.now()
	var row sqlc.GetOpenInvitationRow
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if _, err := s.lockInvitation(ctx, q, org, id, actor); err != nil {
			return err
		}
		if row, err = q.GetOpenInvitation(ctx, sqlc.GetOpenInvitationParams{ID: id, OrganizationID: org}); err != nil {
			return err
		}
		_, err := q.RevokeInvitation(ctx, sqlc.RevokeInvitationParams{ID: id, OrganizationID: org, Now: now})
		return err
	})
	if err != nil {
		return models.Invitation{}, err
	}
	out := invitationOf(row, now)
	return out, s.audit(ctx, eventbus.IdentityInvitationRevoked, []uuid.UUID{org}, targetInvitation, id.String(),
		map[string]string{"Email": out.Email, "Role": out.Role})
}

func (s *Service) ResendInvitation(ctx context.Context, org, id uuid.UUID) (models.InvitationIssued, error) {
	p, actor, err := actorIn(ctx, org)
	if err != nil {
		return models.InvitationIssued{}, err
	}
	token, err := newInviteToken()
	if err != nil {
		return models.InvitationIssued{}, err
	}
	now := s.now()
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		row, err := s.lockInvitation(ctx, q, org, id, actor)
		if err != nil {
			return err
		}
		if row.SendCount >= config.InvitationMaxSends {
			return apperrors.InvitationSendLimit()
		}
		if err := s.takeInviteLimits(ctx, org, p.UserID); err != nil {
			return err
		}
		_, err = q.ReissueInvitation(ctx, sqlc.ReissueInvitationParams{
			ID: id, OrganizationID: org, TokenHash: token.hash, ExpiresAt: now.Add(config.InvitationTTL), Now: now,
			EmailStatus: s.initialEmailStatus(), InvitedByUserID: p.UserID,
		})
		return err
	})
	if err != nil {
		return models.InvitationIssued{}, err
	}
	return s.deliver(ctx, org, id, token, eventbus.IdentityInvitationResent)
}
