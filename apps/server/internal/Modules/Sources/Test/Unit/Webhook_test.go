package unit_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sources "github.com/Khaymah/dhow/apps/server/internal/Modules/Sources"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Sources/Models"
)

var (
	secret = []byte("the webhook secret of repository one")
	other  = []byte("the webhook secret of repository two")
	now    = time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	sha    = "0123456789abcdef0123456789abcdef01234567"
)

func sign(key []byte, parts ...string) string {
	h := hmac.New(sha256.New, key)
	for _, p := range parts {
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func dhowHeaders(key []byte, at time.Time, body string) http.Header {
	t := strconv.FormatInt(at.Unix(), 10)
	h := http.Header{}
	h.Set(sources.HeaderDhowSignature, "t="+t+",v1="+sign(key, t, ".", body))
	h.Set(sources.HeaderDhowDelivery, "d-1")
	return h
}

func giteaHeaders(name string, key []byte, body string) http.Header {
	h := http.Header{}
	value := sign(key, body)
	if name == sources.HeaderHubSignature {
		value = "sha256=" + value
	}
	h.Set(name, value)
	h.Set(sources.HeaderGiteaDelivery, "8c5a3f2e-1111-4222-8333-944445555666")
	h.Set(sources.HeaderGiteaEvent, "push")
	return h
}

func pushBody(ref, after string) string {
	return `{"ref":"` + ref + `","before":"` + strings.Repeat("1", 40) + `","after":"` + after +
		`","pusher":{"login":"sara","email":"sara@example.com"},"repository":{"full_name":"team/app"}}`
}

func verify(h http.Header, key []byte, body string) (sources.Proof, error) {
	return sources.VerifyGenericWebhook(sources.WebhookRequest{Header: h, Body: []byte(body), Secret: key, Now: now})
}

func TestTheFourSignatureFormatsAreAccepted(t *testing.T) {
	body := pushBody("refs/heads/main", sha)
	for _, tt := range []struct {
		header string
		format string
	}{
		{sources.HeaderForgejoSignature, sources.FormatForgejo},
		{sources.HeaderGiteaSignature, sources.FormatGitea},
		{sources.HeaderHubSignature, sources.FormatHub},
	} {
		proof, err := verify(giteaHeaders(tt.header, secret, body), secret, body)
		require.NoError(t, err, tt.header)
		assert.Equal(t, sources.Proof{Format: tt.format, DeliveryID: "8c5a3f2e-1111-4222-8333-944445555666"}, proof)
	}
	ours := `{"Ref":"refs/heads/main","Sha":"` + sha + `"}`
	proof, err := verify(dhowHeaders(secret, now.Add(-4*time.Minute), ours), secret, ours)
	require.NoError(t, err)
	assert.Equal(t, sources.Proof{Format: sources.FormatDhow, DeliveryID: "d-1"}, proof)
}

func TestBadSignaturesAreRefusedAlike(t *testing.T) {
	body := pushBody("refs/heads/main", sha)
	ours := `{"Ref":"refs/heads/main","Sha":"` + sha + `"}`
	cases := map[string]struct {
		h    http.Header
		body string
	}{
		"no signature": {http.Header{sources.HeaderGiteaDelivery: {"x"}}, body},
		"a wrong hmac": {func() http.Header {
			h := giteaHeaders(sources.HeaderGiteaSignature, secret, body)
			h.Set(sources.HeaderGiteaSignature, strings.Repeat("ab", 32))
			return h
		}(), body},
		"the secret of another repo": {giteaHeaders(sources.HeaderGiteaSignature, other, body), body},
		"an altered body":            {giteaHeaders(sources.HeaderGiteaSignature, secret, body), strings.Replace(body, "main", "mail", 1)},
		"one byte more":              {giteaHeaders(sources.HeaderForgejoSignature, secret, body), body + " "},
		"hub without prefix": {func() http.Header {
			h := http.Header{}
			h.Set(sources.HeaderHubSignature, sign(secret, body))
			h.Set(sources.HeaderGiteaDelivery, "x")
			return h
		}(), body},
		"not hex": {func() http.Header {
			h := giteaHeaders(sources.HeaderGiteaSignature, secret, body)
			h.Set(sources.HeaderGiteaSignature, "zz")
			return h
		}(), body},
		"two signature values": {func() http.Header {
			h := giteaHeaders(sources.HeaderGiteaSignature, secret, body)
			h.Add(sources.HeaderGiteaSignature, "x")
			return h
		}(), body},
		"a timestamp older than 5 min":    {dhowHeaders(secret, now.Add(-5*time.Minute-time.Second), ours), ours},
		"a timestamp 6 minutes ahead":     {dhowHeaders(secret, now.Add(6*time.Minute), ours), ours},
		"our format with another secret":  {dhowHeaders(other, now, ours), ours},
		"our format with an altered body": {dhowHeaders(secret, now, ours), strings.Replace(ours, "main", "evil", 1)},
		"a forged timestamp": {func() http.Header {
			h := dhowHeaders(secret, now.Add(-time.Hour), ours)
			h.Set(sources.HeaderDhowSignature, strings.Replace(h.Get(sources.HeaderDhowSignature), strconv.FormatInt(now.Add(-time.Hour).Unix(), 10), strconv.FormatInt(now.Unix(), 10), 1))
			return h
		}(), ours},
		"two timestamps": {func() http.Header {
			h := dhowHeaders(secret, now, ours)
			h.Set(sources.HeaderDhowSignature, h.Get(sources.HeaderDhowSignature)+",t=1")
			return h
		}(), ours},
		"no timestamp": {func() http.Header {
			h := http.Header{}
			h.Set(sources.HeaderDhowSignature, "v1="+sign(secret, ours))
			h.Set(sources.HeaderDhowDelivery, "x")
			return h
		}(), ours},
		"a timestamp with a +": {func() http.Header {
			h := http.Header{}
			ts := "+" + strconv.FormatInt(now.Unix(), 10)
			h.Set(sources.HeaderDhowSignature, "t="+ts+",v1="+sign(secret, ts, ".", ours))
			h.Set(sources.HeaderDhowDelivery, "x")
			return h
		}(), ours},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := verify(c.h, secret, c.body)
			require.ErrorIs(t, err, sources.ErrSignature)
		})
	}
}

func TestADeliveryIdIsRequiredAfterAGoodSignature(t *testing.T) {
	body := pushBody("refs/heads/main", sha)
	h := giteaHeaders(sources.HeaderGiteaSignature, secret, body)
	h.Del(sources.HeaderGiteaDelivery)
	_, err := verify(h, secret, body)
	require.ErrorIs(t, err, sources.ErrDeliveryID)
	h.Set(sources.HeaderGiteaDelivery, "bad id with spaces")
	_, err = verify(h, secret, body)
	require.ErrorIs(t, err, sources.ErrDeliveryID)
	h.Del(sources.HeaderGiteaDelivery)
	h.Set(sources.HeaderForgejoDelivery, "forgejo-1")
	proof, err := verify(h, secret, body)
	require.NoError(t, err)
	assert.Equal(t, "forgejo-1", proof.DeliveryID)

	ours := `{"Ref":"refs/heads/main","Sha":"` + sha + `"}`
	d := dhowHeaders(secret, now, ours)
	d.Del(sources.HeaderDhowDelivery)
	d.Set(sources.HeaderGiteaDelivery, "x")
	_, err = verify(d, secret, ours)
	require.ErrorIs(t, err, sources.ErrDeliveryID)
}

func parse(t *testing.T, h http.Header, body string) (models.Event, error) {
	t.Helper()
	req := sources.WebhookRequest{Header: h, Body: []byte(body), Secret: secret, Now: now}
	proof, err := sources.VerifyGenericWebhook(req)
	require.NoError(t, err)
	return sources.ParseGenericEvent(req, proof)
}

func TestPayloads(t *testing.T) {
	ev, err := parse(t, giteaHeaders(sources.HeaderGiteaSignature, secret, pushBody("refs/heads/main", sha)), pushBody("refs/heads/main", sha))
	require.NoError(t, err)
	assert.Equal(t, models.Event{Name: "push", Ref: "refs/heads/main", Sha: sha, Actor: "sara"}, ev)

	tag := pushBody("refs/tags/v1.0.0", sha)
	ev, err = parse(t, giteaHeaders(sources.HeaderForgejoSignature, secret, tag), tag)
	require.NoError(t, err)
	assert.Equal(t, "tag", ev.Name)

	deleted := pushBody("refs/heads/old", strings.Repeat("0", 40))
	ev, err = parse(t, giteaHeaders(sources.HeaderGiteaSignature, secret, deleted), deleted)
	require.NoError(t, err)
	assert.Equal(t, models.ReasonRefDeleted, ev.Ignore)
	assert.Empty(t, ev.Sha)

	issue := `{"action":"opened"}`
	h := giteaHeaders(sources.HeaderGiteaSignature, secret, issue)
	h.Set(sources.HeaderGiteaEvent, "issues")
	ev, err = parse(t, h, issue)
	require.NoError(t, err)
	assert.Equal(t, models.Event{Name: "issues", Ignore: models.ReasonEventNotSupported}, ev)

	ours := `{"Ref":"refs/tags/v2","Sha":"` + sha + `"}`
	ev, err = parse(t, dhowHeaders(secret, now, ours), ours)
	require.NoError(t, err)
	assert.Equal(t, models.Event{Name: "tag", Ref: "refs/tags/v2", Sha: sha}, ev)
}

func TestBadPayloadsAreRefused(t *testing.T) {
	for name, body := range map[string]string{
		"not json":           `nope`,
		"an option as a ref": pushBody("--upload-pack=x", sha),
		"a dash branch":      pushBody("refs/heads/-oProxyCommand", sha),
		"two dots":           pushBody("refs/heads/a..b", sha),
		"reflog syntax":      pushBody("refs/heads/a@{1}", sha),
		"a newline":          pushBody(`refs/heads/a\nb`, sha),
		"a pull request ref": pushBody("refs/pull/1/head", sha),
		"a short sha":        pushBody("refs/heads/main", "abc"),
		"an upper sha":       pushBody("refs/heads/main", strings.ToUpper(sha)),
		"trailing data":      pushBody("refs/heads/main", sha) + `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parse(t, giteaHeaders(sources.HeaderGiteaSignature, secret, body), body)
			require.ErrorIs(t, err, sources.ErrPayload)
		})
	}
	for name, body := range map[string]string{
		"an unknown field":   `{"Ref":"refs/heads/main","Sha":"` + sha + `","Script":"x"}`,
		"a missing sha":      `{"Ref":"refs/heads/main"}`,
		"an array":           `[]`,
		"two objects":        `{"Ref":"refs/heads/main","Sha":"` + sha + `"}{}`,
		"a control in a ref": `{"Ref":"refs/heads/a\u0000","Sha":"` + sha + `"}`,
	} {
		t.Run("ours/"+name, func(t *testing.T) {
			_, err := parse(t, dhowHeaders(secret, now, body), body)
			require.ErrorIs(t, err, sources.ErrPayload)
		})
	}
}

func TestCleanActor(t *testing.T) {
	assert.Equal(t, "sara[2J", sources.CleanActor(" sara\x1b[2J "))
	assert.Equal(t, "abc", sources.CleanActor("a\u202eb\u200bc"))
	assert.Empty(t, sources.CleanActor("\xff"))
	assert.Len(t, []rune(sources.CleanActor(strings.Repeat("ع", 300))), 255)
}
