package alerts

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	i18n "github.com/Khaymah/dhow/apps/server/internal/Core/I18n"
	mailer "github.com/Khaymah/dhow/apps/server/internal/Core/Mail"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
)

type Options struct {
	DB        sqlc.DBTX
	Mailer    mailer.Sender
	PublicURL string
	Log       *slog.Logger
	Now       func() time.Time
}

type Service struct {
	db      sqlc.DBTX
	mailer  mailer.Sender
	link    string
	log     *slog.Logger
	now     func() time.Time
	slots   chan struct{}
	mu      sync.Mutex
	closed  bool
	pending sync.WaitGroup
}

func NewService(o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		db: o.DB, mailer: o.Mailer, link: o.PublicURL + config.AccountSecurityPath, log: o.Log, now: now,
		slots: make(chan struct{}, config.AlertMaxPending),
	}
}

func (s *Service) Handle(ctx context.Context, ev eventbus.Event) error {
	ctx = context.WithoutCancel(ctx)
	change, ok := changeOf(ev.Name)
	if !ok || !s.mailer.Enabled() {
		return nil
	}
	a, ok := ev.Payload.(eventbus.Audited)
	if !ok {
		s.log.WarnContext(ctx, "credential alert skipped: unexpected payload", "event", ev.Name)
		return nil
	}
	user, ok := recipientOf(ev.Name, a)
	if !ok {
		s.log.WarnContext(ctx, "credential alert skipped: no recipient", "event", ev.Name)
		return nil
	}
	r, err := sqlc.New(s.db).GetAlertRecipient(ctx, user)
	if err != nil {
		s.log.WarnContext(ctx, "credential alert skipped: recipient lookup failed", "event", ev.Name, "user_id", user.String(), "error", err)
		return nil
	}
	msg := Message(i18n.Locale(r.Locale), Details{
		To: r.Email, Name: r.Name, Change: change, At: s.now(), IP: a.IP, UserAgent: auth.RequestMetaFrom(ctx).UserAgent, Link: s.link,
	})
	s.send(ctx, ev.Name, user, msg)
	return nil
}

func (s *Service) send(ctx context.Context, event string, user uuid.UUID, msg mailer.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		s.log.WarnContext(ctx, "credential alert not sent: shutting down", "event", event, "user_id", user.String())
		return
	}
	select {
	case s.slots <- struct{}{}:
	default:
		s.log.WarnContext(ctx, "credential alert not sent: too many pending", "event", event, "user_id", user.String())
		return
	}
	s.pending.Go(func() {
		defer func() { <-s.slots }()
		if err := s.mailer.Send(ctx, msg); err != nil {
			s.log.WarnContext(ctx, "credential alert not sent", "event", event, "user_id", user.String(), "error", err)
		}
	})
}

func (s *Service) Wait() { s.pending.Wait() }

func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.pending.Wait()
}

func recipientOf(event string, a eventbus.Audited) (uuid.UUID, bool) {
	var raw string
	switch event {
	case eventbus.IdentityMemberMfaReset:
		raw = a.TargetID
	case eventbus.AuthApiTokenCreated:
		raw = a.Data[ownerKey]
	default:
		if a.ActorUserID == nil {
			return uuid.Nil, false
		}
		return *a.ActorUserID, true
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, false
	}
	return id, true
}

func Subscribe(bus *eventbus.Bus, s *Service) {
	for _, name := range Events() {
		bus.Subscribe(name, s.Handle)
	}
}
