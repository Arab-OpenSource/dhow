package identity

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	i18n "github.com/Khaymah/dhow/apps/server/internal/Core/I18n"
	mailer "github.com/Khaymah/dhow/apps/server/internal/Core/Mail"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Identity/Models"
)

const (
	keyInviteSubject = "mail.invite.subject"
	keyInviteBody    = "mail.invite.body"
	mailDateLayout   = "2006-01-02 15:04"
)

func roleLabel(locale i18n.Locale, role string) string { return i18n.Translate(locale, "role."+role) }

func inviteMessage(locale i18n.Locale, to, inviter, org, role, link string, expires time.Time) mailer.Message {
	r := strings.NewReplacer(
		"{inviter}", inviter, "{organization}", org, "{role}", roleLabel(locale, role), "{link}", link,
		"{expires}", expires.UTC().Format(mailDateLayout),
	)
	subject := strings.NewReplacer("\r", " ", "\n", " ").Replace(r.Replace(i18n.Translate(locale, keyInviteSubject)))
	return mailer.Message{To: to, Subject: subject, Body: r.Replace(i18n.Translate(locale, keyInviteBody))}
}

func (s *Service) sendInvitation(ctx context.Context, org uuid.UUID, row sqlc.GetOpenInvitationRow, link string) string {
	if !s.mailer.Enabled() {
		return models.EmailDisabled
	}
	o, err := s.queries().GetOrganizationByID(ctx, org)
	if err != nil {
		s.log.ErrorContext(ctx, "invitation email not sent: organization lookup failed", "error", err)
		return models.EmailFailed
	}
	msg := inviteMessage(i18n.LocaleFrom(ctx), row.Email, row.InvitedByName, o.Name, row.Role, link, row.ExpiresAt)
	if err := s.mailer.Send(ctx, msg); err != nil {
		s.log.WarnContext(ctx, "invitation email not sent", "invitation_id", row.Id.String(), "error", err)
		return models.EmailFailed
	}
	return models.EmailSent
}
