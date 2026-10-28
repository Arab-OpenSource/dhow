package app_test

import (
	"net/http"
	"net/netip"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "github.com/Arab-OpenSource/dhow/apps/server/internal/App"
	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
)

const testPublicLimit = 3

type limitedRoute struct {
	path string
	body map[string]string
}

var (
	invitationPreview = limitedRoute{"/invitations/preview", map[string]string{"Token": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}}
	invitationAccept  = limitedRoute{"/invitations/accept", map[string]string{"Token": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "Name": "Sara", "Password": "correct horse battery staple"}}
	oidcAuthorize     = limitedRoute{"/oidc/authorize", map[string]string{"Provider": "corp"}}
	oidcCallback      = limitedRoute{"/oidc/callback", map[string]string{"Provider": "corp", "Code": "c", "State": "s"}}
	passkeyOptions    = limitedRoute{"/sessions/passkey-options", nil}
	setup             = limitedRoute{"/setup", map[string]string{"SetupToken": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "OwnerName": "O", "Email": "o@example.com", "Password": "correct horse battery staple", "OrganizationName": "O", "OrganizationSlug": "o"}}
)

func limitedEnv(t *testing.T, configure func(*config.Config)) *apptest.Env {
	t.Helper()
	return apptest.NewWithOptions(t, apptest.Options{Profile: func(c *config.Config) {
		c.RateLimits.Public = testPublicLimit
		if configure != nil {
			configure(c)
		}
	}})
}

func hit(env *apptest.Env, r limitedRoute, remote string, headers map[string]string) apptest.Response {
	h := map[string]string{"Origin": apptest.PublicOrigin}
	for k, v := range headers {
		h[k] = v
	}
	req := apptest.Request{Method: http.MethodPost, Path: r.path, Headers: h, RemoteAddr: remote}
	if r.body != nil {
		req.Body = r.body
	}
	return env.Do(req)
}

func assertLimited(t *testing.T, res apptest.Response, why ...string) {
	t.Helper()
	require.Equal(t, http.StatusTooManyRequests, res.Status, "%v: %s", why, res.Raw)
	require.NotNil(t, res.Error)
	assert.Equal(t, apperrors.CodeRateLimited, res.Error.Code)
	retry, err := strconv.Atoi(res.Header.Get("Retry-After"))
	require.NoError(t, err, "Retry-After is whole seconds")
	assert.Positive(t, retry)
	assert.LessOrEqual(t, retry, int(config.RateLimitPublicWindow.Seconds()))
}

func TestTheRateLimitedRoutesArePublicAndMatchThePlan(t *testing.T) {
	rules := map[string]policy.Rule{}
	for _, r := range app.Policies() {
		rules[r.Key()] = r
	}
	var got []string
	for _, r := range app.RateLimited() {
		rule, ok := rules[r.Key()]
		require.True(t, ok, "%s is rate limited but has no route", r.Key())
		assert.Equal(t, policy.ScopePublic, rule.Scope, "%s", r.Key())
		got = append(got, r.Key())
	}
	assert.ElementsMatch(t, []string{
		"POST /invitations/preview", "POST /invitations/accept", "POST /oidc/authorize", "POST /oidc/callback",
		"POST /sessions/passkey-options", "POST /setup",
	}, got)
}

func TestEveryLimitedRouteAnswers429AtTheLimit(t *testing.T) {
	for _, r := range []limitedRoute{invitationPreview, invitationAccept, oidcAuthorize, oidcCallback, passkeyOptions, setup} {
		t.Run(r.path, func(t *testing.T) {
			env := limitedEnv(t, nil)
			for i := range testPublicLimit {
				res := hit(env, r, "", nil)
				require.NotEqual(t, http.StatusTooManyRequests, res.Status, "request %d: %s", i+1, res.Raw)
			}
			res := hit(env, r, "", nil)
			assertLimited(t, res)
			assert.Equal(t, "طلبات كتير ورا بعض. استنى شوية وجرّب تاني.", res.Error.Message)
			en := hit(env, r, "", map[string]string{"Accept-Language": "en"})
			assertLimited(t, en)
			assert.Equal(t, "Too many requests in a row. Wait a little and try again.", en.Error.Message)

			env.Clock.Advance(config.RateLimitPublicWindow)
			assert.NotEqual(t, http.StatusTooManyRequests, hit(env, r, "", nil).Status, "the next window starts again")
		})
	}
}

func TestRoutesOfOneGroupShareTheirCounter(t *testing.T) {
	for name, pair := range map[string][2]limitedRoute{
		"invitations": {invitationPreview, invitationAccept},
		"oidc":        {oidcAuthorize, oidcCallback},
	} {
		t.Run(name, func(t *testing.T) {
			env := limitedEnv(t, nil)
			for range testPublicLimit {
				require.NotEqual(t, http.StatusTooManyRequests, hit(env, pair[0], "", nil).Status)
			}
			assertLimited(t, hit(env, pair[1], "", nil))
			assert.NotEqual(t, http.StatusTooManyRequests, hit(env, passkeyOptions, "", nil).Status, "other groups count apart")
		})
	}
}

func TestAForgedForwardedForWithoutATrustedProxyChangesNothing(t *testing.T) {
	env := limitedEnv(t, nil)
	for i := range testPublicLimit + 3 {
		res := hit(env, oidcAuthorize, "198.51.100.20:5000", map[string]string{"X-Forwarded-For": "192.0.2." + strconv.Itoa(i+1)})
		if i < testPublicLimit {
			require.NotEqual(t, http.StatusTooManyRequests, res.Status)
			continue
		}
		assertLimited(t, res)
	}
}

func TestATrustedProxyCountsEachClientApart(t *testing.T) {
	env := limitedEnv(t, func(c *config.Config) { c.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")} })
	for i := range testPublicLimit {
		require.NotEqual(t, http.StatusTooManyRequests, hit(env, oidcAuthorize, "10.0.0.5:5000", map[string]string{"X-Forwarded-For": "192.0.2.1"}).Status, "request %d", i)
	}
	assertLimited(t, hit(env, oidcAuthorize, "10.0.0.5:5000", map[string]string{"X-Forwarded-For": "192.0.2.1"}))
	assert.NotEqual(t, http.StatusTooManyRequests, hit(env, oidcAuthorize, "10.0.0.5:5000", map[string]string{"X-Forwarded-For": "192.0.2.2"}).Status,
		"another client behind the same proxy has its own counter")
	assertLimited(t, hit(env, oidcAuthorize, "10.0.0.6:5000", map[string]string{"X-Forwarded-For": "203.0.113.50, 192.0.2.1"}),
		"the left-most hop is not trusted, the right-most untrusted one counts")
}

func TestIPv6ClientsAreCountedPerSlash64(t *testing.T) {
	env := limitedEnv(t, nil)
	for i := range testPublicLimit {
		remote := "[2001:db8:1:2::" + strconv.Itoa(i+1) + "]:443"
		require.NotEqual(t, http.StatusTooManyRequests, hit(env, passkeyOptions, remote, nil).Status)
	}
	assertLimited(t, hit(env, passkeyOptions, "[2001:db8:1:2:ffff:ffff:ffff:ffff]:443", nil))
	assert.NotEqual(t, http.StatusTooManyRequests, hit(env, passkeyOptions, "[2001:db8:1:3::1]:443", nil).Status, "the next /64 is another client")
}

func TestASecondInstanceOnTheSameDatabaseSeesTheSameCounter(t *testing.T) {
	first := limitedEnv(t, nil)
	second := apptest.NewWithOptions(t, apptest.Options{Pool: first.Pool, Clock: first.Clock, Profile: func(c *config.Config) {
		c.RateLimits.Public = testPublicLimit
	}})
	require.NotSame(t, first.Wired, second.Wired)
	for i := range testPublicLimit {
		env := first
		if i%2 == 1 {
			env = second
		}
		require.NotEqual(t, http.StatusTooManyRequests, hit(env, invitationPreview, "", nil).Status)
	}
	assertLimited(t, hit(first, invitationPreview, "", nil))
	assertLimited(t, hit(second, invitationPreview, "", nil))
}

func TestACrossSitePageCannotSpendTheLimitOfAnOffice(t *testing.T) {
	for _, r := range []limitedRoute{invitationPreview, oidcAuthorize, passkeyOptions, setup} {
		t.Run(r.path, func(t *testing.T) {
			env := limitedEnv(t, nil)
			for _, origin := range []string{"https://evil.example", "null", ""} {
				for range testPublicLimit + 2 {
					res := hit(env, r, "", map[string]string{"Origin": origin})
					require.Equal(t, http.StatusForbidden, res.Status, "origin %q: %s", origin, res.Raw)
					require.Equal(t, apperrors.CodeCSRFFailed, res.Error.Code)
				}
			}
			for i := range testPublicLimit {
				assert.NotEqual(t, http.StatusTooManyRequests, hit(env, r, "", nil).Status, "same-origin request %d still has the whole limit", i+1)
			}
			assertLimited(t, hit(env, r, "", nil))
		})
	}
}
