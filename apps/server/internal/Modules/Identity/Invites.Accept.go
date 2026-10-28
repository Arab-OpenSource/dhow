package identity

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	i18n "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/I18n"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Tx"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Identity/Models"
)

const constraintUserEmail = "UX_Users_Email"

func inviterStillAllowed(ctx context.Context, q *sqlc.Queries, org, inviter uuid.UUID, role string) (bool, error) {
	m, err := loadMember(ctx, q, org, inviter)
	if isNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return policy.Allows(m.Effective, policy.MemberManage) && policy.CanGrant(m.Effective, policy.Role(role)), nil
}

func (s *Service) usableInvitation(ctx context.Context, q *sqlc.Queries, token string) (sqlc.FindUsableInvitationRow, error) {
	hash, ok := decodeInviteToken(token)
	if !ok {
		return sqlc.FindUsableInvitationRow{}, apperrors.InvitationInvalid()
	}
	row, err := q.FindUsableInvitation(ctx, sqlc.FindUsableInvitationParams{TokenHash: hash, Now: s.now()})
	if errors.Is(err, pgx.ErrNoRows) {
		return row, apperrors.InvitationInvalid()
	}
	if err != nil {
		return row, err
	}
	allowed, err := inviterStillAllowed(ctx, q, row.OrganizationId, row.InvitedByUserId, row.Role)
	if err != nil {
		return row, err
	}
	if !allowed {
		return row, apperrors.InvitationInvalid()
	}
	return row, nil
}

func (s *Service) PreviewInvitation(ctx context.Context, token string) (models.InvitationPreview, error) {
	row, err := s.usableInvitation(ctx, s.queries(), token)
	if err != nil {
		return models.InvitationPreview{}, err
	}
	return models.InvitationPreview{
		OrganizationName: row.OrganizationName, InviterName: row.InviterName, Email: row.Email, Role: row.Role,
		ExpiresAt: row.ExpiresAt, AccountExists: row.AccountExists,
	}, nil
}

func (s *Service) claim(ctx context.Context, q *sqlc.Queries, token string) (sqlc.LockUsableInvitationRow, error) {
	hash, ok := decodeInviteToken(token)
	if !ok {
		return sqlc.LockUsableInvitationRow{}, apperrors.InvitationInvalid()
	}
	row, err := q.LockUsableInvitation(ctx, sqlc.LockUsableInvitationParams{TokenHash: hash, Now: s.now()})
	if errors.Is(err, pgx.ErrNoRows) {
		return row, apperrors.InvitationInvalid()
	}
	if err != nil {
		return row, err
	}
	allowed, err := inviterStillAllowed(ctx, q, row.OrganizationId, row.InvitedByUserId, row.Role)
	if err != nil {
		return row, err
	}
	if !allowed {
		return row, apperrors.InvitationInvalid()
	}
	return row, nil
}

func join(ctx context.Context, q *sqlc.Queries, inv sqlc.LockUsableInvitationRow, user uuid.UUID, now func() time.Time) (models.Organization, error) {
	t := now()
	if err := q.InsertMembership(ctx, sqlc.InsertMembershipParams{UserID: user, OrganizationID: inv.OrganizationId, Role: inv.Role, Now: t}); err != nil {
		return models.Organization{}, err
	}
	n, err := q.MarkInvitationAccepted(ctx, sqlc.MarkInvitationAcceptedParams{ID: inv.Id, UserID: &user, Now: t})
	if err != nil {
		return models.Organization{}, err
	}
	if n == 0 {
		return models.Organization{}, apperrors.InvitationInvalid()
	}
	o, err := q.GetOrganizationByID(ctx, inv.OrganizationId)
	if err != nil {
		return models.Organization{}, err
	}
	return models.Organization{ID: o.Id, Name: o.Name, Slug: o.Slug, CreatedAt: o.CreatedAt}, nil
}

func validateAccept(in *models.AcceptInput) *apperrors.AppError {
	var details []apperrors.IErrorDetail
	if !auth.ValidDisplayName(in.Name, maxNameRunes) {
		details = append(details, invalid(fieldName))
	}
	if !validLocale(in.Locale) {
		details = append(details, invalid(fieldLocale))
	}
	if len(details) > 0 {
		return apperrors.ValidationFailed(details...)
	}
	return nil
}

func (s *Service) AcceptInvitation(ctx context.Context, in models.AcceptInput) (models.InvitationAccepted, error) {
	if in.Locale == "" {
		in.Locale = string(i18n.LocaleFrom(ctx))
	}
	if ae := validateAccept(&in); ae != nil {
		return models.InvitationAccepted{}, ae
	}
	pre, err := s.usableInvitation(ctx, s.queries(), in.Token)
	if err != nil {
		return models.InvitationAccepted{}, err
	}
	if pre.AccountExists {
		return models.InvitationAccepted{}, apperrors.InvitationAccountExists()
	}
	if ae := s.passwords.Validate(ctx, fieldPassword, in.Password); ae != nil {
		return models.InvitationAccepted{}, ae
	}
	hash, err := s.hasher.Hash(ctx, in.Password)
	if err != nil {
		return models.InvitationAccepted{}, err
	}
	var out models.InvitationAccepted
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		inv, err := s.claim(ctx, q, in.Token)
		if err != nil {
			return err
		}
		user, err := q.InsertUser(ctx, sqlc.InsertUserParams{Email: inv.Email, Name: in.Name, PasswordHash: hash, Locale: in.Locale, Now: s.now()})
		if err != nil {
			return err
		}
		org, err := join(ctx, q, inv, user.Id, s.now)
		if err != nil {
			return err
		}
		out = models.InvitationAccepted{
			User: models.User{ID: user.Id, Email: user.Email, Name: user.Name, Locale: user.Locale}, Organization: org, Role: inv.Role,
		}
		return nil
	})
	if uniqueConstraint(err) == constraintUserEmail {
		return models.InvitationAccepted{}, apperrors.InvitationAccountExists()
	}
	if err != nil {
		return models.InvitationAccepted{}, err
	}
	return out, s.auditAs(ctx, out.User.ID, eventbus.IdentityInvitationAccepted, []uuid.UUID{out.Organization.ID}, targetUser,
		out.User.ID.String(), map[string]string{"Email": out.User.Email, "Role": out.Role, "NewAccount": "true"})
}

func (s *Service) AcceptInvitationSignedIn(ctx context.Context, token string) (models.InvitationAccepted, error) {
	p, err := principal(ctx)
	if err != nil {
		return models.InvitationAccepted{}, err
	}
	var out models.InvitationAccepted
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		inv, err := s.claim(ctx, q, token)
		if err != nil {
			return err
		}
		if auth.NormalizeEmail(p.Email) != inv.Email {
			return apperrors.InvitationEmailMismatch()
		}
		if _, err := loadMember(ctx, q, inv.OrganizationId, p.UserID); err == nil {
			return apperrors.AlreadyMember()
		} else if !isNotFound(err) {
			return err
		}
		org, err := join(ctx, q, inv, p.UserID, s.now)
		if err != nil {
			return err
		}
		out = models.InvitationAccepted{
			User: models.User{ID: p.UserID, Email: p.Email, Name: p.Name, Locale: p.Locale}, Organization: org, Role: inv.Role,
		}
		return nil
	})
	if err != nil {
		return models.InvitationAccepted{}, err
	}
	return out, s.audit(ctx, eventbus.IdentityInvitationAccepted, []uuid.UUID{out.Organization.ID}, targetUser, p.UserID.String(),
		map[string]string{"Email": out.User.Email, "Role": out.Role, "NewAccount": "false"})
}
