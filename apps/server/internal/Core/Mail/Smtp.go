package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	netmail "net/mail"
	"net/smtp"
	"strconv"
	"time"
)

const (
	TLSStartTLS = "starttls"
	TLSImplicit = "tls"
	TLSNone     = "none"
)

var (
	ErrDisabled    = errors.New("email is not configured")
	ErrNoStartTLS  = errors.New("the mail server does not offer STARTTLS")
	ErrUnknownMode = errors.New("unknown SMTP TLS mode")
)

type Sender interface {
	Enabled() bool
	Send(ctx context.Context, m Message) error
}

type Disabled struct{}

func (Disabled) Enabled() bool { return false }

func (Disabled) Send(context.Context, Message) error { return ErrDisabled }

type SMTPOptions struct {
	Host      string
	Port      int
	Username  string
	Password  string
	From      netmail.Address
	TLS       string
	Timeout   time.Duration
	TLSConfig *tls.Config
	Now       func() time.Time
}

type SMTP struct {
	o SMTPOptions
}

func NewSMTP(o SMTPOptions) *SMTP {
	if o.Now == nil {
		o.Now = time.Now
	}
	return &SMTP{o: o}
}

func (s *SMTP) Enabled() bool { return true }

func (s *SMTP) tlsConfig() *tls.Config {
	cfg := &tls.Config{ServerName: s.o.Host, MinVersion: tls.VersionTLS12}
	if s.o.TLSConfig != nil {
		cfg = s.o.TLSConfig.Clone()
		cfg.ServerName = s.o.Host
		cfg.MinVersion = tls.VersionTLS12
	}
	return cfg
}

func (s *SMTP) Send(ctx context.Context, m Message) error {
	body, err := build(s.o.From, m, s.o.Now())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, s.o.Timeout)
	defer cancel()
	conn, err := s.dial(ctx)
	if err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			_ = conn.Close()
			return fmt.Errorf("set mail deadline: %w", err)
		}
	}
	c, err := smtp.NewClient(conn, s.o.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("greet mail server: %w", err)
	}
	defer func() { _ = c.Close() }()
	return s.deliver(c, m.To, body)
}

func (s *SMTP) dial(ctx context.Context) (net.Conn, error) {
	addr := net.JoinHostPort(s.o.Host, strconv.Itoa(s.o.Port))
	d := &net.Dialer{}
	switch s.o.TLS {
	case TLSImplicit:
		conn, err := (&tls.Dialer{NetDialer: d, Config: s.tlsConfig()}).DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("connect to mail server: %w", err)
		}
		return conn, nil
	case TLSStartTLS, TLSNone:
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("connect to mail server: %w", err)
		}
		return conn, nil
	default:
		return nil, ErrUnknownMode
	}
}

func (s *SMTP) deliver(c *smtp.Client, to string, body []byte) error {
	if s.o.TLS == TLSStartTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return ErrNoStartTLS
		}
		if err := c.StartTLS(s.tlsConfig()); err != nil {
			return fmt.Errorf("start TLS: %w", err)
		}
	}
	if s.o.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", s.o.Username, s.o.Password, s.o.Host)); err != nil {
			return fmt.Errorf("authenticate to mail server: %w", err)
		}
	}
	if err := c.Mail(s.o.From.Address); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("finish message: %w", err)
	}
	if err := c.Quit(); err != nil {
		return fmt.Errorf("quit: %w", err)
	}
	return nil
}
