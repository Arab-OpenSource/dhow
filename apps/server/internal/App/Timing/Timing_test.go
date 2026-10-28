//go:build timing

package timing_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	mathrand "math/rand/v2"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
)

const (
	rounds = 200
	warmup = 20
)

type tolerance struct {
	relative float64
	absolute time.Duration
}

var (
	medianTolerance   = tolerance{relative: 0.10, absolute: 300 * time.Microsecond}
	quartileTolerance = tolerance{relative: 0.20, absolute: 600 * time.Microsecond}
)

type probe struct {
	name   string
	status int
	code   string
	send   func(round int) apptest.Response
}

type stats struct {
	p25, p50, p75 time.Duration
}

func quantile(sorted []time.Duration, q float64) time.Duration {
	return sorted[int(q*float64(len(sorted)-1))]
}

func summarize(samples []time.Duration) stats {
	sorted := slices.Clone(samples)
	slices.Sort(sorted)
	return stats{p25: quantile(sorted, 0.25), p50: quantile(sorted, 0.5), p75: quantile(sorted, 0.75)}
}

func within(got, ref time.Duration, tol tolerance) bool {
	diff := got - ref
	if diff < 0 {
		diff = -diff
	}
	return diff <= max(time.Duration(tol.relative*float64(ref)), tol.absolute)
}

func measure(t *testing.T, between func(round int), probes ...probe) map[string]stats {
	t.Helper()
	samples := make(map[string][]time.Duration, len(probes))
	order := make([]int, len(probes))
	for i := range order {
		order[i] = i
	}
	for round := range warmup + rounds {
		if between != nil {
			between(round)
		}
		mathrand.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		for _, i := range order {
			p := probes[i]
			start := time.Now()
			res := p.send(round)
			elapsed := time.Since(start)
			require.Equal(t, p.status, res.Status, "%s: %s", p.name, res.Raw)
			require.NotNil(t, res.Error, p.name)
			require.Equal(t, p.code, res.Error.Code, p.name)
			if round >= warmup {
				samples[p.name] = append(samples[p.name], elapsed)
			}
		}
	}
	out := make(map[string]stats, len(probes))
	for name, s := range samples {
		out[name] = summarize(s)
	}
	return out
}

func assertIndistinguishable(t *testing.T, got map[string]stats, reference string) {
	t.Helper()
	ref := got[reference]
	t.Logf("%-40s p25 %-10v median %-10v p75 %v", reference+" (reference)", ref.p25, ref.p50, ref.p75)
	for name, s := range got {
		if name == reference {
			continue
		}
		t.Logf("%-40s p25 %-10v median %-10v p75 %v", name, s.p25, s.p50, s.p75)
		if !within(s.p50, ref.p50, medianTolerance) {
			t.Errorf("%s: the median %v differs from %v of %s beyond the tolerance", name, s.p50, ref.p50, reference)
		}
		if !within(s.p25, ref.p25, quartileTolerance) || !within(s.p75, ref.p75, quartileTolerance) {
			t.Errorf("%s: the quartiles %v..%v differ from %v..%v of %s beyond the tolerance", name, s.p25, s.p75, ref.p25, ref.p75, reference)
		}
	}
}

func clientAddress(round, client int) string {
	return fmt.Sprintf("198.%d.%d.%d:4000", 18+client, round/250, 1+(round%250))
}

func randomToken(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(b)
}

func unlimited(c *config.Config) { c.RateLimits.Public = config.RateLimitUpperBound }

func TestTimingSignInDoesNotRevealWhetherAnAccountExists(t *testing.T) {
	params := auth.DefaultParams()
	env := apptest.NewWithOptions(t, apptest.Options{Params: &params, Profile: unlimited})
	_, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	disabled := env.SeedUser(org, "disabled@example.com", "a long password for disabled", "viewer")
	_, err := env.Pool.Exec(context.Background(), `UPDATE "Users" SET "Disabled" = true WHERE "Id" = $1`, disabled)
	require.NoError(t, err)

	login := func(name, email, password string, n int) probe {
		return probe{name: name, status: http.StatusUnauthorized, code: "INVALID_CREDENTIALS", send: func(round int) apptest.Response {
			return env.Do(apptest.Request{
				Method: http.MethodPost, Path: "/sessions", RemoteAddr: clientAddress(round, n),
				Headers: map[string]string{"Origin": apptest.PublicOrigin},
				Body:    map[string]string{"Email": email, "Password": password},
			})
		}}
	}
	got := measure(t, func(int) { env.Clock.Advance(config.ThrottleResetAfter + config.ThrottleMaxLock) },
		login("unknown account", "nobody@example.com", "a wrong password here", 0),
		login("existing account, wrong password", d.Email, "a wrong password here", 1),
		login("disabled account, right password", "disabled@example.com", "a long password for disabled", 2),
	)
	assertIndistinguishable(t, got, "unknown account")
}

func TestTimingAnInvitationTokenDoesNotRevealWhyItIsInvalid(t *testing.T) {
	env := apptest.NewWithOptions(t, apptest.Options{Profile: unlimited})
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	expired := owner.MustInvite(org, "expired@example.com", "viewer")
	revoked := owner.MustInvite(org, "revoked@example.com", "viewer")
	used := owner.MustInvite(org, "used@example.com", "viewer")
	ctx := context.Background()
	_, err := env.Pool.Exec(ctx, `UPDATE "Invitations" SET "ExpiresAt" = $2 WHERE "Id" = $1`, expired.Invitation.Id, env.Clock.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, owner.Do(apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(org, "/invitations/"+revoked.Invitation.Id)}).Status)
	require.Equal(t, http.StatusOK, env.AcceptNew(used.Token, "Used", "a long password for used@example.com").Status)

	preview := func(name, token string, n int) probe {
		return probe{name: name, status: http.StatusGone, code: "INVITATION_INVALID", send: func(round int) apptest.Response {
			return env.Do(apptest.Request{
				Method: http.MethodPost, Path: "/invitations/preview", RemoteAddr: clientAddress(round, n),
				Headers: map[string]string{"Origin": apptest.PublicOrigin}, Body: map[string]string{"Token": token},
			})
		}}
	}
	got := measure(t, nil,
		preview("unknown token", randomToken(t), 0),
		preview("expired token", expired.Token, 1),
		preview("revoked token", revoked.Token, 2),
		preview("used token", used.Token, 3),
	)
	assertIndistinguishable(t, got, "unknown token")
}

func TestTimingAnAPITokenDoesNotRevealWhetherItExists(t *testing.T) {
	env := apptest.NewWithOptions(t, apptest.Options{Profile: unlimited})
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	user := owner.Me().User.Id
	revoked := owner.MustCreateAPIToken(org, []string{"org.read"}, d.Password)
	expired := owner.MustCreateAPIToken(org, []string{"org.read"}, d.Password)
	ctx := context.Background()
	now := env.Clock.Now()
	_, err := env.Pool.Exec(ctx, `UPDATE "ApiTokens" SET "RevokedAt" = $2, "RevokedByUserId" = $3 WHERE "Id" = $1`, revoked.ID, now, user)
	require.NoError(t, err)
	_, err = env.Pool.Exec(ctx, `UPDATE "ApiTokens" SET "CreatedAt" = $2, "ExpiresAt" = $3 WHERE "Id" = $1`, expired.ID, now.Add(-48*time.Hour), now.Add(-time.Hour))
	require.NoError(t, err)
	tokens, err := auth.NewAPITokens(env.Config.SecretKey)
	require.NoError(t, err)
	unknown, _, err := tokens.New()
	require.NoError(t, err)

	bearer := func(name, token string) probe {
		return probe{name: name, status: http.StatusUnauthorized, code: "UNAUTHENTICATED", send: func(int) apptest.Response {
			return env.Bearer(token, apptest.Request{Method: http.MethodGet, Path: apptest.OrgPath(org, "")})
		}}
	}
	got := measure(t, nil,
		bearer("unknown token", unknown),
		bearer("revoked token", revoked.Value),
		bearer("expired token", expired.Value),
	)
	assertIndistinguishable(t, got, "unknown token")
}

func TestTimingTheMethodNoticesOneExtraMillisecond(t *testing.T) {
	env := apptest.NewWithOptions(t, apptest.Options{Profile: unlimited})
	token := randomToken(t)
	preview := func(round, client int) apptest.Response {
		return env.Do(apptest.Request{
			Method: http.MethodPost, Path: "/invitations/preview", RemoteAddr: clientAddress(round, client),
			Headers: map[string]string{"Origin": apptest.PublicOrigin}, Body: map[string]string{"Token": token},
		})
	}
	got := measure(t, nil,
		probe{name: "plain", status: http.StatusGone, code: "INVITATION_INVALID", send: func(round int) apptest.Response { return preview(round, 0) }},
		probe{name: "one millisecond slower", status: http.StatusGone, code: "INVITATION_INVALID", send: func(round int) apptest.Response {
			res := preview(round, 1)
			time.Sleep(time.Millisecond)
			return res
		}},
	)
	require.False(t, within(got["one millisecond slower"].p50, got["plain"].p50, medianTolerance),
		"the tolerance %v is too loose to notice a 1ms leak: %v against %v", medianTolerance, got["one millisecond slower"].p50, got["plain"].p50)
}

func TestTimingTheMethodNoticesASignInThatIsSlowerThanTheTolerance(t *testing.T) {
	params := auth.DefaultParams()
	env := apptest.NewWithOptions(t, apptest.Options{Params: &params, Profile: unlimited})
	require.Equal(t, http.StatusOK, env.Setup(apptest.DefaultSetup()).Status)
	login := func(round, client int) apptest.Response {
		return env.Do(apptest.Request{
			Method: http.MethodPost, Path: "/sessions", RemoteAddr: clientAddress(round, client),
			Headers: map[string]string{"Origin": apptest.PublicOrigin},
			Body:    map[string]string{"Email": "nobody@example.com", "Password": "a wrong password here"},
		})
	}
	const extra = 6 * time.Millisecond
	got := measure(t, func(int) { env.Clock.Advance(config.ThrottleResetAfter + config.ThrottleMaxLock) },
		probe{name: "plain", status: http.StatusUnauthorized, code: "INVALID_CREDENTIALS", send: func(round int) apptest.Response { return login(round, 0) }},
		probe{name: "slower", status: http.StatusUnauthorized, code: "INVALID_CREDENTIALS", send: func(round int) apptest.Response {
			res := login(round, 1)
			time.Sleep(extra)
			return res
		}},
	)
	plain, slower := got["plain"].p50, got["slower"].p50
	t.Logf("sign-in median %v, with %v more %v, tolerance %v", plain, extra, slower, max(time.Duration(medianTolerance.relative*float64(plain)), medianTolerance.absolute))
	require.Less(t, time.Duration(medianTolerance.relative*float64(plain)), extra, "the sign-in path is slower than this control assumes")
	require.False(t, within(slower, plain, medianTolerance), "a sign-in %v slower must be noticed: %v against %v", extra, slower, plain)
}
