package instancesettings

import (
	"context"
	"crypto/tls"
	"time"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	mailer "github.com/Khaymah/dhow/apps/server/internal/Core/Mail"
)

type Mailer struct {
	store     *Store
	tlsConfig *tls.Config
	now       func() time.Time
}

var _ mailer.Sender = (*Mailer)(nil)

func NewMailer(store *Store, tlsConfig *tls.Config, now func() time.Time) *Mailer {
	return &Mailer{store: store, tlsConfig: tlsConfig, now: now}
}

func (m *Mailer) Enabled() bool { return m.store.Current().Values.Mail.Enabled() }

func (m *Mailer) Send(ctx context.Context, msg mailer.Message) error {
	cfg := m.store.Current().Values.Mail
	if !cfg.Enabled() {
		return mailer.ErrDisabled
	}
	return mailer.NewSMTP(mailer.SMTPOptions{
		Host: cfg.Host, Port: cfg.Port, Username: cfg.Username, Password: cfg.Password, From: cfg.From,
		TLS: cfg.TLS, Timeout: config.MailTimeout, TLSConfig: m.tlsConfig, Now: m.now,
	}).Send(ctx, msg)
}

type BreachChecker interface {
	IsBreached(ctx context.Context, password string) (bool, error)
}

type Breach struct {
	store   *Store
	checker BreachChecker
}

func NewBreach(store *Store, checker BreachChecker) *Breach {
	return &Breach{store: store, checker: checker}
}

func (b *Breach) IsBreached(ctx context.Context, password string) (bool, error) {
	if !b.store.Current().Values.BreachedPasswordCheck {
		return false, nil
	}
	return b.checker.IsBreached(ctx, password)
}
