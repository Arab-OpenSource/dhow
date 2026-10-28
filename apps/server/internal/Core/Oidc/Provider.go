package oidcauth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

const (
	RequestTimeout      = 10 * time.Second
	DiscoveryRetryAfter = 30 * time.Second
	MaxResponseBytes    = 1 << 20
	MaxSubjectLength    = 255
)

var (
	ErrUnavailable = errors.New("identity provider is unavailable")
	ErrRejected    = errors.New("identity provider response was rejected")
	ErrUnknown     = errors.New("unknown identity provider")
	errTooLarge    = errors.New("identity provider response is too large")
	errNoRedirects = errors.New("identity provider redirects are not followed")
	errUnsupported = errors.New("unsupported identity provider type")
)

const errStatusPrefix = "identity provider answered"

type Info struct {
	ID   string
	Name string
	Kind string
}

type Email struct {
	Address  string
	Verified bool
	Primary  bool
}

type Identity struct {
	Issuer  string
	Subject string
	Name    string
	Emails  []Email
}

type Flow struct {
	State       string
	Nonce       string
	Verifier    string
	RedirectURL string
}

type Provider interface {
	Info() Info
	Issuer() string
	AllowedDomains() []string
	AuthorizeURL(ctx context.Context, f Flow) (string, error)
	Exchange(ctx context.Context, code string, f Flow) (Identity, error)
}

type Options struct {
	Transport http.RoundTripper
	Now       func() time.Time
	AllowHTTP bool
}

type Registry struct {
	order     []string
	providers map[string]Provider
}

func NewRegistry(cfgs []config.OIDCProvider, o Options) (*Registry, error) {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	client := newClient(o.Transport)
	r := &Registry{providers: make(map[string]Provider, len(cfgs))}
	for _, c := range cfgs {
		var p Provider
		switch c.Kind {
		case config.OIDCKindOIDC:
			p = newOIDC(c, client, now, o.AllowHTTP)
		case config.OIDCKindGitHub:
			p = newGitHub(c, client)
		default:
			return nil, fmt.Errorf("%w: %s", errUnsupported, c.Kind)
		}
		r.order = append(r.order, c.ID)
		r.providers[c.ID] = p
	}
	return r, nil
}

func (r *Registry) Get(id string) (Provider, error) {
	if r == nil {
		return nil, ErrUnknown
	}
	p, ok := r.providers[id]
	if !ok {
		return nil, ErrUnknown
	}
	return p, nil
}

func (r *Registry) List() []Info {
	if r == nil {
		return []Info{}
	}
	out := make([]Info, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.providers[id].Info())
	}
	return out
}

func newClient(base http.RoundTripper) *http.Client {
	if base == nil {
		base = http.DefaultTransport
	}
	return &http.Client{
		Timeout:   RequestTimeout,
		Transport: limitedTransport{base: base},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errNoRedirects
		},
	}
}

type limitedTransport struct {
	base http.RoundTripper
}

func (t limitedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	res, err := t.base.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	res.Body = &limitedBody{rc: res.Body, left: MaxResponseBytes}
	return res, nil
}

type limitedBody struct {
	rc   io.ReadCloser
	left int64
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.left <= 0 {
		var one [1]byte
		if n, _ := b.rc.Read(one[:]); n > 0 {
			return 0, errTooLarge
		}
		return 0, io.EOF
	}
	if int64(len(p)) > b.left {
		p = p[:b.left]
	}
	n, err := b.rc.Read(p)
	b.left -= int64(n)
	return n, err
}

func (b *limitedBody) Close() error { return b.rc.Close() }

func rejected(step string, err error) error {
	return fmt.Errorf("%w: %s: %w", ErrRejected, step, err)
}

func readJSON(res *http.Response) ([]byte, error) {
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s %d", errStatusPrefix, res.StatusCode)
	}
	return body, nil
}
