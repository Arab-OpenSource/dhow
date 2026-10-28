package identity

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	mailer "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Mail"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Tx"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Identity/Models"
)

const uniqueViolation = "23505"

type DB interface {
	sqlc.DBTX
	dbtx.Beginner
}

type Publisher interface {
	Publish(ctx context.Context, event eventbus.Event) error
}

type Options struct {
	DB        DB
	Hasher    *auth.Hasher
	Passwords *auth.PasswordPolicy
	Bus       Publisher
	Factors   auth.FactorConfirmer
	Mailer    mailer.Sender
	PublicURL string
	Log       *slog.Logger
	Now       func() time.Time
}

type Service struct {
	db        DB
	hasher    *auth.Hasher
	passwords *auth.PasswordPolicy
	bus       Publisher
	factors   auth.FactorConfirmer
	mailer    mailer.Sender
	publicURL string
	log       *slog.Logger
	now       func() time.Time
}

func NewService(o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	mail := o.Mailer
	if mail == nil {
		mail = mailer.Disabled{}
	}
	return &Service{
		db: o.DB, hasher: o.Hasher, passwords: o.Passwords, bus: o.Bus, factors: o.Factors, mailer: mail,
		publicURL: o.PublicURL, log: o.Log, now: now,
	}
}

func (s *Service) queries() *sqlc.Queries { return sqlc.New(s.db) }

func (s *Service) EnsureSetupToken(ctx context.Context) (string, bool, error) {
	inst, err := s.queries().GetInstance(ctx)
	if err != nil {
		return "", false, err
	}
	if inst.SetupCompletedAt != nil {
		return "", false, nil
	}
	raw := make([]byte, config.SetupTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", false, fmt.Errorf("read random bytes: %w", err)
	}
	now := s.now()
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if err := q.InvalidateSetupTokens(ctx, now); err != nil {
			return err
		}
		return q.InsertSetupToken(ctx, sqlc.InsertSetupTokenParams{TokenHash: auth.HashBytes(raw), ExpiresAt: now.Add(config.SetupTokenTTL), Now: now})
	})
	if err != nil {
		return "", false, err
	}
	return s.publicURL + "/setup#" + base64.RawURLEncoding.EncodeToString(raw), true, nil
}

func decodeSetupToken(v string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(v)
	if err != nil || len(raw) != config.SetupTokenBytes {
		return nil, false
	}
	return auth.HashBytes(raw), true
}

func (s *Service) Setup(ctx context.Context, in models.SetupInput) (models.SetupResult, error) {
	if ae := validateSetup(&in); ae != nil {
		return models.SetupResult{}, ae
	}
	tokenHash, ok := decodeSetupToken(in.Token)
	if !ok {
		return models.SetupResult{}, apperrors.SetupTokenInvalid()
	}
	if err := s.precheck(ctx, tokenHash); err != nil {
		return models.SetupResult{}, err
	}
	if ae := s.passwords.Validate(ctx, fieldPassword, in.Password); ae != nil {
		return models.SetupResult{}, ae
	}
	hash, err := s.hasher.Hash(ctx, in.Password)
	if err != nil {
		return models.SetupResult{}, err
	}
	res, err := s.create(ctx, in, tokenHash, hash)
	if err != nil {
		return models.SetupResult{}, err
	}
	s.announce(ctx, res)
	return res, nil
}

func (s *Service) precheck(ctx context.Context, tokenHash []byte) error {
	q := s.queries()
	inst, err := q.GetInstance(ctx)
	if err != nil {
		return err
	}
	if inst.SetupCompletedAt != nil {
		return apperrors.SetupCompleted()
	}
	_, err = q.GetUsableSetupToken(ctx, sqlc.GetUsableSetupTokenParams{TokenHash: tokenHash, Now: s.now()})
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if inst, err = q.GetInstance(ctx); err != nil {
		return err
	}
	if inst.SetupCompletedAt != nil {
		return apperrors.SetupCompleted()
	}
	return apperrors.SetupTokenInvalid()
}

func (s *Service) create(ctx context.Context, in models.SetupInput, tokenHash []byte, passwordHash string) (models.SetupResult, error) {
	var res models.SetupResult
	now := s.now()
	err := dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		completedAt, err := q.LockInstance(ctx)
		if err != nil {
			return err
		}
		if completedAt != nil {
			return apperrors.SetupCompleted()
		}
		tokenID, err := q.LockUsableSetupToken(ctx, sqlc.LockUsableSetupTokenParams{TokenHash: tokenHash, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.SetupTokenInvalid()
		}
		if err != nil {
			return err
		}
		org, err := q.InsertOrganization(ctx, sqlc.InsertOrganizationParams{Name: in.OrganizationName, Slug: in.OrganizationSlug, Now: now})
		if err != nil {
			return err
		}
		user, err := q.InsertUser(ctx, sqlc.InsertUserParams{Email: in.Email, Name: in.OwnerName, PasswordHash: passwordHash, Locale: in.Locale, Now: now})
		if err != nil {
			return err
		}
		err = q.InsertMembership(ctx, sqlc.InsertMembershipParams{UserID: user.Id, OrganizationID: org.Id, Role: string(policy.RoleOwner), Now: now})
		if err != nil {
			return err
		}
		if err := q.MarkInstanceOwner(ctx, sqlc.MarkInstanceOwnerParams{ID: user.Id, Now: now}); err != nil {
			return err
		}
		if err := q.MarkSetupTokenUsed(ctx, sqlc.MarkSetupTokenUsedParams{ID: tokenID, Now: now}); err != nil {
			return err
		}
		if err := q.CompleteInstanceSetup(ctx, now); err != nil {
			return err
		}
		res = models.SetupResult{
			User:         models.User{ID: user.Id, Email: user.Email, Name: user.Name, Locale: user.Locale},
			Organization: models.Organization{ID: org.Id, Name: org.Name, Slug: org.Slug, CreatedAt: org.CreatedAt},
		}
		return nil
	})
	if err != nil {
		return models.SetupResult{}, mapUnique(err)
	}
	return res, nil
}

func mapUnique(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != uniqueViolation {
		return err
	}
	switch pgErr.ConstraintName {
	case "UX_Organizations_Slug":
		return conflict(fieldOrganizationSlug, messageSlugTaken)
	case "UX_Users_Email":
		return conflict(fieldEmail, messageEmailTaken)
	default:
		return apperrors.Conflict()
	}
}

func conflict(field, message string) *apperrors.AppError {
	e := apperrors.Conflict()
	e.Details = []apperrors.IErrorDetail{{Field: field, Message: message}}
	return e
}

func (s *Service) announce(ctx context.Context, res models.SetupResult) {
	meta := auth.RequestMetaFrom(ctx)
	actor := res.User.ID
	err := s.bus.Publish(ctx, eventbus.Event{Name: eventbus.SystemInstanceSetupCompleted, Payload: eventbus.Audited{
		OrganizationIDs: []uuid.UUID{res.Organization.ID}, ActorUserID: &actor, TargetType: "organization",
		TargetID: res.Organization.ID.String(), IP: meta.IPString(), Data: map[string]string{"OrganizationSlug": res.Organization.Slug},
	}})
	if err != nil {
		s.log.ErrorContext(ctx, "setup completed but the audit event was not recorded", "error", err)
	}
}
