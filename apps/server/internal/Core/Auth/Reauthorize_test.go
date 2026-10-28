package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
)

type liveResolver struct {
	mu       sync.Mutex
	sessions map[string]auth.Resolved
	err      error
	peeks    int
}

func (l *liveResolver) Resolve(_ context.Context, hash []byte, _ time.Time) (auth.Resolved, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err != nil {
		return auth.Resolved{}, l.err
	}
	r, ok := l.sessions[string(hash)]
	if !ok {
		return auth.Resolved{}, auth.ErrNoSession
	}
	return r, nil
}

func (l *liveResolver) Peek(ctx context.Context, hash []byte, now time.Time) (auth.Resolved, error) {
	l.mu.Lock()
	l.peeks++
	l.mu.Unlock()
	return l.Resolve(ctx, hash, now)
}

func (l *liveResolver) set(hash []byte, r *auth.Resolved, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.err = err
	if r == nil {
		delete(l.sessions, string(hash))
		return
	}
	l.sessions[string(hash)] = *r
}

func TestReauthorizeFollowsTheSessionAndTheRole(t *testing.T) {
	org := uuid.New()
	cookie, hash, err := auth.NewSessionToken(config.SessionTokenBytes)
	require.NoError(t, err)
	resolver := &liveResolver{sessions: map[string]auth.Resolved{}}
	u, err := url.Parse(origin)
	require.NoError(t, err)
	guard := auth.NewGuard(auth.GuardOptions{
		Resolver: resolver, CSRF: auth.NewCSRF([]byte("0123456789abcdef0123456789abcdef")), PublicURL: u,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)),
	})
	ctx := auth.WithRequestMeta(context.Background(), auth.RequestMeta{SessionToken: cookie})
	member := func(role policy.Role) *auth.Resolved {
		return &auth.Resolved{UserID: uuid.New(), SessionID: uuid.New(), Memberships: []policy.Membership{{OrganizationID: org, Role: role}}}
	}
	codeOf := func(err error) string {
		var ae *apperrors.AppError
		if errors.As(err, &ae) {
			return ae.Code
		}
		return ""
	}

	resolver.set(hash, member(policy.RoleDeveloper), nil)
	require.NoError(t, guard.Reauthorize(ctx, policy.LogRead, org))
	assert.Equal(t, 1, resolver.peeks, "re-authorization peeks and never resolves, so it does not touch the session")

	tests := []struct {
		name     string
		resolved *auth.Resolved
		storeErr error
		org      uuid.UUID
		want     string
	}{
		{"the role dropped to viewer", member(policy.RoleViewer), nil, org, apperrors.CodeForbidden},
		{"the membership was removed", member(policy.RoleOwner), nil, uuid.New(), apperrors.CodeNotFound},
		{"the session was revoked", nil, nil, org, apperrors.CodeUnauthenticated},
		{"the session store fails", member(policy.RoleOwner), errors.New("database is down"), org, apperrors.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver.set(hash, tt.resolved, tt.storeErr)
			assert.Equal(t, tt.want, codeOf(guard.Reauthorize(ctx, policy.LogRead, tt.org)))
		})
	}

	resolver.set(hash, member(policy.RoleOwner), nil)
	assert.Equal(t, apperrors.CodeUnauthenticated, codeOf(guard.Reauthorize(context.Background(), policy.LogRead, org)),
		"a context without the session cookie never passes")
}
