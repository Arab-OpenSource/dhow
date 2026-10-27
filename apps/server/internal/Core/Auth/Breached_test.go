package auth_test

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
)

func digest(password string) string {
	sum := sha1.Sum([]byte(password))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func TestMatchRangeAgainstTheFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/hibp-range.txt")
	require.NoError(t, err)
	assert.Equal(t, "5BAA6", digest("password")[:5])
	tests := []struct {
		name   string
		suffix string
		want   bool
	}{
		{"a breached password", digest("password")[5:], true},
		{"lower-case suffix still matches", strings.ToLower(digest("password")[5:]), true},
		{"not in the range", digest("a long unique passphrase")[5:], false},
		{"empty suffix", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := auth.MatchRange(bytes.NewReader(fixture), tt.suffix)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchRangeIgnoresPadding(t *testing.T) {
	suffix := digest("padded entry")[5:]
	body := suffix + ":0\r\n" + digest("another")[5:] + ":4\r\n"
	got, err := auth.MatchRange(strings.NewReader(body), suffix)
	require.NoError(t, err)
	assert.False(t, got, "a padding line has a count of zero")
}

func TestMatchRangeRejectsGarbageCounts(t *testing.T) {
	suffix := digest("x")[5:]
	_, err := auth.MatchRange(strings.NewReader(suffix+":many\n"), suffix)
	assert.Error(t, err)
}

type capture struct {
	mu      sync.Mutex
	path    string
	padding string
	agent   string
	query   string
	calls   int
}

func (c *capture) handler(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		c.path, c.padding, c.agent, c.query, c.calls = r.URL.Path, r.Header.Get("Add-Padding"), r.UserAgent(), r.URL.RawQuery, c.calls+1
		c.mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestHIBPSendsOnlyThePrefix(t *testing.T) {
	fixture, err := os.ReadFile("testdata/hibp-range.txt")
	require.NoError(t, err)
	c := &capture{}
	srv := httptest.NewServer(c.handler(http.StatusOK, string(fixture)))
	defer srv.Close()
	h := auth.NewHIBP(srv.URL+"/range/", time.Second, nil)

	breached, err := h.IsBreached(context.Background(), "password")
	require.NoError(t, err)
	assert.True(t, breached)
	assert.Equal(t, "/range/5BAA6", c.path)
	assert.Equal(t, "true", c.padding)
	assert.NotEmpty(t, c.agent)
	assert.Empty(t, c.query)
	assert.NotContains(t, c.path, digest("password")[5:], "the suffix must never leave the server")
	assert.NotContains(t, c.path, "password")

	breached, err = h.IsBreached(context.Background(), "a long unique passphrase")
	require.NoError(t, err)
	assert.False(t, breached)
	assert.Equal(t, "/range/"+digest("a long unique passphrase")[:5], c.path)
}

func TestHIBPFailureModes(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer slow.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.1:1/elsewhere", http.StatusFound)
	}))
	defer redirect.Close()
	c := &capture{}
	failing := httptest.NewServer(c.handler(http.StatusServiceUnavailable, "down"))
	defer failing.Close()
	limited := httptest.NewServer(c.handler(http.StatusTooManyRequests, "slow down"))
	defer limited.Close()

	tests := []struct {
		name string
		url  string
	}{
		{"timeout", slow.URL + "/range/"},
		{"redirect is not followed", redirect.URL + "/range/"},
		{"server error", failing.URL + "/range/"},
		{"rate limited", limited.URL + "/range/"},
		{"nothing listening", "http://127.0.0.1:1/range/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := auth.NewHIBP(tt.url, 150*time.Millisecond, nil)
			breached, err := h.IsBreached(context.Background(), "password")
			assert.Error(t, err)
			assert.False(t, breached)
			if err != nil {
				assert.NotContains(t, err.Error(), "5BAA6", "the hash prefix is not logged")
			}
		})
	}
}

type failingChecker struct{ err error }

func (f failingChecker) IsBreached(context.Context, string) (bool, error) { return false, f.err }

type fixedChecker struct{ breached bool }

func (f fixedChecker) IsBreached(context.Context, string) (bool, error) { return f.breached, nil }

func TestPasswordPolicy(t *testing.T) {
	arabic12 := strings.Repeat("س", 12)
	tests := []struct {
		name     string
		checker  auth.BreachChecker
		password string
		wantCode string
		wantWarn bool
	}{
		{"valid", fixedChecker{}, "correct horse battery staple", "", false},
		{"exactly 12 characters", fixedChecker{}, "abcdefghijkl", "", false},
		{"12 multi-byte characters count as 12", fixedChecker{}, arabic12, "", false},
		{"11 characters", fixedChecker{}, "abcdefghijk", apperrors.CodeValidationFailed, false},
		{"exactly 128 characters", fixedChecker{}, strings.Repeat("a", 128), "", false},
		{"129 characters", fixedChecker{}, strings.Repeat("a", 129), apperrors.CodeValidationFailed, false},
		{"empty", fixedChecker{}, "", apperrors.CodeValidationFailed, false},
		{"breached", fixedChecker{breached: true}, "correct horse battery staple", apperrors.CodePasswordBreached, false},
		{"the service is down: allow and warn", failingChecker{err: errors.New("timeout")}, "correct horse battery staple", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			p := auth.NewPasswordPolicy(tt.checker, slog.New(slog.NewJSONHandler(&logs, nil)), 12, 128)
			ae := p.Validate(context.Background(), "Password", tt.password)
			if tt.wantCode == "" {
				assert.Nil(t, ae)
			} else if assert.NotNil(t, ae) {
				assert.Equal(t, tt.wantCode, ae.Code)
			}
			assert.Equal(t, tt.wantWarn, strings.Contains(logs.String(), `"level":"WARN"`))
			assert.NotContains(t, logs.String(), tt.password+"\"", "the password is never logged")
		})
	}
}

func TestPasswordPolicyReportsTheField(t *testing.T) {
	p := auth.NewPasswordPolicy(fixedChecker{}, slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)), 12, 128)
	ae := p.Validate(context.Background(), "NewPassword", "short")
	require.NotNil(t, ae)
	require.Len(t, ae.Details, 1)
	assert.Equal(t, "NewPassword", ae.Details[0].Field)
	assert.Equal(t, "errors.passwordLength", ae.Details[0].Message)
}

func TestNoBreachCheck(t *testing.T) {
	breached, err := auth.NoBreachCheck{}.IsBreached(context.Background(), "password")
	require.NoError(t, err)
	assert.False(t, breached)
}
