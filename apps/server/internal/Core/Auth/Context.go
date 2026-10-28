package auth

import (
	"context"
	"net/netip"
	"time"

	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
)

type Principal struct {
	policy.Subject
	Email     string
	Name      string
	Locale    string
	CSRFToken string
	ExpiresAt time.Time
}

type RequestMeta struct {
	IP           netip.Addr
	UserAgent    string
	SessionToken string
	MfaToken     string
	OidcToken    string
}

type principalKey struct{}

type metaKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

func SubjectFrom(ctx context.Context) (policy.Subject, bool) {
	p, ok := PrincipalFrom(ctx)
	return p.Subject, ok
}

func WithRequestMeta(ctx context.Context, m RequestMeta) context.Context {
	return context.WithValue(ctx, metaKey{}, m)
}

func RequestMetaFrom(ctx context.Context) RequestMeta {
	m, _ := ctx.Value(metaKey{}).(RequestMeta)
	return m
}

func (m RequestMeta) IPString() string {
	if !m.IP.IsValid() {
		return ""
	}
	return m.IP.String()
}
