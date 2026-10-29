package settings

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	i18n "github.com/Khaymah/dhow/apps/server/internal/Core/I18n"
	mailer "github.com/Khaymah/dhow/apps/server/internal/Core/Mail"
	ratelimit "github.com/Khaymah/dhow/apps/server/internal/Core/RateLimit"
	instancesettings "github.com/Khaymah/dhow/apps/server/internal/Core/Settings"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
)

const (
	targetSettings  = "instance_settings"
	limitMailTest   = "mail-test"
	keyTestSubject  = "mail.settingsTest.subject"
	keyTestBody     = "mail.settingsTest.body"
	fieldListMaxLen = 200
)

type Publisher interface {
	Publish(ctx context.Context, event eventbus.Event) error
}

type Options struct {
	Store   *instancesettings.Store
	DB      sqlc.DBTX
	Factors auth.FactorConfirmer
	Bus     Publisher
	Mailer  mailer.Sender
	Limiter ratelimit.Limiter
	Log     *slog.Logger
	Now     func() time.Time
}

type Service struct {
	store   *instancesettings.Store
	db      sqlc.DBTX
	factors auth.FactorConfirmer
	bus     Publisher
	mailer  mailer.Sender
	limiter ratelimit.Limiter
	log     *slog.Logger
	now     func() time.Time
}

func NewService(o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	limiter := o.Limiter
	if limiter == nil {
		limiter = ratelimit.NewMemory(config.RateLimitTrackedKeys, now)
	}
	mail := o.Mailer
	if mail == nil {
		mail = mailer.Disabled{}
	}
	return &Service{
		store: o.Store, db: o.DB, factors: o.Factors, bus: o.Bus, mailer: mail, limiter: limiter, log: o.Log, now: now,
	}
}

func owner(ctx context.Context) (auth.Principal, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return auth.Principal{}, apperrors.Unauthenticated()
	}
	if !p.InstanceOwner || p.ViaToken() {
		return auth.Principal{}, apperrors.Forbidden()
	}
	return p, nil
}

func (s *Service) Get(ctx context.Context) (*instancesettings.Snapshot, error) {
	if _, err := owner(ctx); err != nil {
		return nil, err
	}
	return s.store.Current(), nil
}

func (s *Service) Update(
	ctx context.Context, g settingspec.Group, version int64, changes []instancesettings.Change, factor auth.Factor,
) (*instancesettings.Snapshot, error) {
	p, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.store.Check(g, version, changes); err != nil {
		return nil, err
	}
	state, err := sqlc.New(s.db).GetUserMfaState(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	if !state.HasTotp && !state.HasPasskey {
		return nil, apperrors.SettingsSecondFactorMissing()
	}
	if err := s.factors.ConfirmFactor(ctx, p, factor); err != nil {
		return nil, err
	}
	res, err := s.store.Update(ctx, g, version, changes, p.UserID)
	if err != nil {
		return nil, err
	}
	if len(res.Changed) == 0 {
		return res.Snapshot, nil
	}
	return res.Snapshot, s.audit(ctx, p.UserID, g, res)
}

func (s *Service) audit(ctx context.Context, actor uuid.UUID, g settingspec.Group, res instancesettings.Result) error {
	fields := strings.Join(res.Changed, ",")
	if len(fields) > fieldListMaxLen {
		fields = fields[:fieldListMaxLen]
	}
	return s.bus.Publish(ctx, eventbus.Event{Name: eventbus.SystemSettingsChanged, Payload: eventbus.Audited{
		ActorUserID: &actor, TargetType: targetSettings, TargetID: string(g), IP: auth.RequestMetaFrom(ctx).IPString(),
		Data: map[string]string{"Group": string(g), "Fields": fields, "Version": strconv.FormatInt(res.Version, 10)},
	}})
}

func (s *Service) SendTest(ctx context.Context) (string, error) {
	p, err := owner(ctx)
	if err != nil {
		return "", err
	}
	d, err := s.limiter.Take(ctx, ratelimit.Limit{Name: limitMailTest, Max: config.MailTestPerWindow, Window: config.MailTestWindow}, p.UserID.String())
	if err != nil {
		return "", err
	}
	if err := d.Err(); err != nil {
		return "", err
	}
	if !s.mailer.Enabled() {
		return "", apperrors.MailNotConfigured()
	}
	locale := i18n.LocaleFrom(ctx)
	msg := mailer.Message{
		To:      p.Email,
		Subject: i18n.Translate(locale, keyTestSubject),
		Body:    strings.ReplaceAll(i18n.Translate(locale, keyTestBody), "{at}", s.now().UTC().Format("2006-01-02 15:04 UTC")),
	}
	if err := s.mailer.Send(ctx, msg); err != nil {
		s.log.WarnContext(ctx, "the test email was not sent", "error", err)
		return "", apperrors.MailTestFailed(nil)
	}
	return p.Email, nil
}
