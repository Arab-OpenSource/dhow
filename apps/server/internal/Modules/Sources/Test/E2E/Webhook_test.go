package e2e_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

func linkHTTPS(t *testing.T, w world) (repoView, string) {
	t.Helper()
	l := linked(t, w.link(t, w.owner, "https://git.example.com/team/app.git"), http.StatusCreated)
	require.NotNil(t, l.WebhookSecret)
	return l.Repository, *l.WebhookSecret
}

func (w world) send(path string, headers map[string]string, body []byte) apptest.Response {
	return w.env.Webhook(http.MethodPost, path, headers, body)
}

func TestASignedPushTriggersOnceAndADuplicateDoesNothing(t *testing.T) {
	w := newWorld(t, false)
	r, secret := linkHTTPS(t, w)
	body := giteaPush("refs/heads/main", sha)
	res := w.send(hookPath(r), giteaHeaders(secret, "delivery-1", body), body)
	require.Equal(t, http.StatusAccepted, res.Status, string(res.Raw))
	assert.Equal(t, "sources.webhook.accepted", res.Message)

	got := w.triggers.Received()
	require.Len(t, got, 1)
	assert.Equal(t, "push", got[0].Event)
	assert.Equal(t, "refs/heads/main", got[0].Ref)
	assert.Equal(t, sha, got[0].Sha)
	assert.Equal(t, "sara", got[0].Actor)
	assert.Equal(t, "generic", got[0].Provider)
	assert.Equal(t, w.project, got[0].ProjectID)
	assert.Equal(t, w.org, got[0].OrganizationID)
	assert.Equal(t, r.Id, got[0].RepositoryID.String())
	assert.NotEqual(t, uuid.Nil, got[0].DeliveryID)

	again := w.send(hookPath(r), giteaHeaders(secret, "delivery-1", body), body)
	require.Equal(t, http.StatusOK, again.Status, string(again.Raw))
	assert.Equal(t, "sources.webhook.duplicate", again.Message)
	replayed := w.send(hookPath(r), giteaHeaders(secret, "delivery-2", body), body)
	require.Equal(t, http.StatusOK, replayed.Status, "the same signed body under a new delivery id is a replay: %s", replayed.Raw)
	assert.Len(t, w.triggers.Received(), 1)
	assert.Equal(t, 1, w.deliveries(t))

	w.env.Clock.Advance(time.Hour)
	assert.Equal(t, http.StatusOK, w.send(hookPath(r), giteaHeaders(secret, "delivery-1", body), body).Status)
	assert.Len(t, w.triggers.Received(), 1)
	w.env.Clock.Advance(10 * 24 * time.Hour)
	late := w.send(hookPath(r), giteaHeaders(secret, "delivery-late", body), body)
	require.Equal(t, http.StatusOK, late.Status, "a body signed without a timestamp stays a replay for the whole retention: %s", late.Raw)
	assert.Len(t, w.triggers.Received(), 1)
	w.env.Clock.Advance(-10*24*time.Hour - time.Hour)

	tag := []byte(`{"Ref":"refs/tags/v1.0.0","Sha":"` + sha + `"}`)
	ours := w.send(hookPath(r), dhowHeaders(secret, "script-1", w.env.Clock.Now().Unix(), tag), tag)
	require.Equal(t, http.StatusAccepted, ours.Status, string(ours.Raw))
	require.Len(t, w.triggers.Received(), 2)
	assert.Equal(t, "tag", w.triggers.Received()[1].Event)
	assert.NotNil(t, repo(t, w.owner.Get(w.path(""))).LastDeliveryAt)
}

func TestBadWebhooksGetTheSame401AndLeaveNoRow(t *testing.T) {
	w := newWorld(t, false)
	r, secret := linkHTTPS(t, w)
	second := world{env: w.env, owner: w.owner, org: w.org, project: w.env.SeedProject(w.org, uuid.MustParse(w.owner.UserID)), triggers: w.triggers}
	_, otherSecret := linkHTTPS(t, second)
	body := giteaPush("refs/heads/main", sha)
	altered := giteaPush("refs/heads/mail", sha)
	now := w.env.Clock.Now().Unix()
	ours := []byte(`{"Ref":"refs/heads/main","Sha":"` + sha + `"}`)
	cases := map[string]struct {
		path    string
		headers map[string]string
		body    []byte
	}{
		"a wrong hmac":                   {hookPath(r), map[string]string{"X-Gitea-Signature": strings.Repeat("ab", 32), "X-Gitea-Delivery": "a"}, body},
		"another repository's secret":    {hookPath(r), giteaHeaders(otherSecret, "b", body), body},
		"an altered body":                {hookPath(r), giteaHeaders(secret, "c", body), altered},
		"a timestamp older than 5 min":   {hookPath(r), dhowHeaders(secret, "d", now-int64(5*time.Minute/time.Second)-1, ours), ours},
		"no signature":                   {hookPath(r), map[string]string{"X-Gitea-Delivery": "e"}, body},
		"an unknown hook":                {config.WebhookPrefix + "/git/" + strings.Repeat("0", 32), giteaHeaders(secret, "f", body), body},
		"a malformed hook":               {config.WebhookPrefix + "/git/..%2f", giteaHeaders(secret, "g", body), body},
		"a signature from the dashboard": {hookPath(r), map[string]string{"X-Gitea-Delivery": "h", "Cookie": "__Host-dhow_session=x"}, body},
	}
	var first string
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			res := w.send(c.path, c.headers, c.body)
			requireError(t, res, http.StatusUnauthorized, "WEBHOOK_REJECTED")
			if first == "" {
				first = string(res.Raw)
			}
			assert.Equal(t, first, string(res.Raw), "every refusal answers the same")
		})
	}
	assert.Zero(t, w.deliveries(t))
	assert.Empty(t, w.triggers.Received())
	logs := w.env.Logs.String()
	for _, s := range []string{secret, otherSecret, mac(secret, string(body)), "X-Gitea-Signature"} {
		assert.NotContains(t, logs, s)
	}
}

func TestIgnoredWebhooksAreRecordedWithoutATrigger(t *testing.T) {
	w := newWorld(t, true)
	r, secret := linkHTTPS(t, w)
	issue := []byte(`{"action":"opened"}`)
	headers := giteaHeaders(secret, "issue-1", issue)
	headers["X-Gitea-Event"] = "issues"
	require.Equal(t, http.StatusAccepted, w.send(hookPath(r), headers, issue).Status)
	deleted := giteaPush("refs/heads/old", strings.Repeat("0", 40))
	require.Equal(t, http.StatusAccepted, w.send(hookPath(r), giteaHeaders(secret, "delete-1", deleted), deleted).Status)

	srv := startSSH(t)
	ssh := world{env: w.env, owner: w.owner, org: w.org, project: w.env.SeedProject(w.org, uuid.MustParse(w.owner.UserID)), triggers: w.triggers}
	l := linked(t, ssh.link(t, w.owner, "ssh://git@git.example.test:"+srv.port+"/app.git"), http.StatusCreated)
	push := giteaPush("refs/heads/main", sha)
	require.Equal(t, http.StatusAccepted, w.send(hookPath(l.Repository), giteaHeaders(*l.WebhookSecret, "ssh-1", push), push).Status)

	_, err := w.env.Pool.Exec(context.Background(), `UPDATE "Projects" SET "State" = 'archived', "ArchivedAt" = now() WHERE "Id" = $1`, w.project)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, w.send(hookPath(r), giteaHeaders(secret, "archived-1", push), push).Status)

	assert.Empty(t, w.triggers.Received())
	reasons := map[string]bool{}
	rows, err := w.env.Pool.Query(context.Background(), `SELECT "Outcome", "Reason" FROM "WebhookDeliveries"`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var outcome, reason string
		require.NoError(t, rows.Scan(&outcome, &reason))
		assert.Equal(t, "ignored", outcome)
		reasons[reason] = true
	}
	assert.Equal(t, map[string]bool{"event_not_supported": true, "ref_deleted": true, "host_key_unconfirmed": true, "project_archived": true}, reasons)
}

func TestAFailingTriggerRollsTheDeliveryBack(t *testing.T) {
	w := newWorld(t, false)
	r, secret := linkHTTPS(t, w)
	body := giteaPush("refs/heads/main", sha)
	w.triggers.FailWith(errors.New("pipelines are down"))
	res := w.send(hookPath(r), giteaHeaders(secret, "rollback-1", body), body)
	requireError(t, res, http.StatusInternalServerError, "INTERNAL")
	assert.Zero(t, w.deliveries(t))

	w.triggers.FailWith(nil)
	require.Equal(t, http.StatusAccepted, w.send(hookPath(r), giteaHeaders(secret, "rollback-1", body), body).Status)
	assert.Len(t, w.triggers.Received(), 1)
	assert.Equal(t, 1, w.deliveries(t))
}

func TestWebhookRequestLimits(t *testing.T) {
	w := newWorld(t, false)
	r, secret := linkHTTPS(t, w)
	body := giteaPush("refs/heads/main", sha)
	missing := giteaHeaders(secret, "", body)
	delete(missing, "X-Gitea-Delivery")
	res := w.send(hookPath(r), missing, body)
	requireError(t, res, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
	assert.Equal(t, "DeliveryId", res.Error.Details[0].Field)

	bad := giteaPush("refs/heads/--upload-pack=x", sha)
	requireError(t, w.send(hookPath(r), giteaHeaders(secret, "bad-ref", bad), bad), http.StatusUnprocessableEntity, "VALIDATION_FAILED")

	huge := make([]byte, config.WebhookMaxBytesDefault+1)
	requireError(t, w.send(hookPath(r), giteaHeaders(secret, "huge", huge), huge), http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE")

	get := w.env.Webhook(http.MethodGet, hookPath(r), nil, nil)
	assert.Equal(t, http.StatusMethodNotAllowed, get.Status, string(get.Raw))
	assert.Zero(t, w.deliveries(t))
}

func TestFailedSignaturesAreLimitedPerHookWithoutStarvingRealDeliveries(t *testing.T) {
	w := newWorld(t, false)
	r, secret := linkHTTPS(t, w)
	var last apptest.Response
	for i := range config.WebhookFailuresPerHook + 1 {
		last = w.env.WebhookFrom(apptest.WebhookRequest{
			Method: http.MethodPost, Path: hookPath(r), Headers: map[string]string{"X-Gitea-Signature": "00", "X-Gitea-Delivery": "x"},
			Body: []byte("{}"), RemoteAddr: "203.0.113." + strconv.Itoa(1+i%200) + ":5000",
		})
	}
	requireError(t, last, http.StatusTooManyRequests, "RATE_LIMITED")
	assert.NotEmpty(t, last.Header.Get("Retry-After"))

	body := giteaPush("refs/heads/main", sha)
	good := w.env.WebhookFrom(apptest.WebhookRequest{
		Method: http.MethodPost, Path: hookPath(r), Headers: giteaHeaders(secret, "real-1", body), Body: body, RemoteAddr: "192.0.2.50:443",
	})
	require.Equal(t, http.StatusAccepted, good.Status, "a viewer who knows the hook id cannot starve real deliveries: %s", good.Raw)
}

func TestTheBucketBeforeAuthenticationIsPerHookAndClient(t *testing.T) {
	w := newWorld(t, false)
	r, secret := linkHTTPS(t, w)
	body := giteaPush("refs/heads/main", sha)
	for i := range config.WebhookPerHookMax {
		b := giteaPush("refs/heads/b"+strconv.Itoa(i), sha)
		res := w.env.WebhookFrom(apptest.WebhookRequest{
			Method: http.MethodPost, Path: hookPath(r), Headers: giteaHeaders(secret, "flood-"+strconv.Itoa(i), b), Body: b,
			RemoteAddr: "[2001:db8:1:1::" + strconv.FormatInt(int64(i%60+1), 16) + "]:5000",
		})
		require.Less(t, res.Status, 300, string(res.Raw))
	}
	same := w.env.WebhookFrom(apptest.WebhookRequest{
		Method: http.MethodPost, Path: hookPath(r), Headers: giteaHeaders(secret, "flood-x", body), Body: body,
		RemoteAddr: "[2001:db8:1:1::ffff]:5000",
	})
	requireError(t, same, http.StatusTooManyRequests, "RATE_LIMITED")
	other := w.env.WebhookFrom(apptest.WebhookRequest{
		Method: http.MethodPost, Path: hookPath(r), Headers: giteaHeaders(secret, "other-1", body), Body: body, RemoteAddr: "192.0.2.50:443",
	})
	assert.Equal(t, http.StatusAccepted, other.Status, string(other.Raw))
}

func TestUnknownHooksUseOnlyThePerAddressBucket(t *testing.T) {
	w := newWorld(t, false)
	for i := range 20 {
		res := w.send(config.WebhookPrefix+"/git/"+strings.Repeat(strconv.Itoa(i%10), 32), map[string]string{"X-Gitea-Signature": "00"}, []byte("{}"))
		requireError(t, res, http.StatusUnauthorized, "WEBHOOK_REJECTED")
	}
	assert.Zero(t, w.env.CountRows(`SELECT count(*) FROM "RateLimits" WHERE "Key" LIKE 'webhook-hook%'`))
	assert.Equal(t, 1, w.env.CountRows(`SELECT count(*) FROM "RateLimits" WHERE "Key" LIKE 'webhook-ip:%'`))
}

func TestBodiesAreReadOnlyForKnownHooksAndWithinTheBudget(t *testing.T) {
	fake := &apptest.FakeTriggers{}
	env := apptest.NewWithOptions(t, apptest.Options{Triggers: fake, GitLookup: lookup, Profile: func(c *config.Config) {
		c.Sources.WebhookInflightBytes = 1024
	}})
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	w := world{env: env, owner: owner, org: org, project: env.SeedProject(org, uuid.MustParse(owner.UserID)), triggers: fake}
	r, secret := linkHTTPS(t, w)

	big := []byte(`{"ref":"refs/heads/main","after":"` + sha + `","pad":"` + strings.Repeat("x", 2048) + `"}`)
	busy := w.send(hookPath(r), giteaHeaders(secret, "big-1", big), big)
	requireError(t, busy, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
	assert.NotEmpty(t, busy.Header.Get("Retry-After"))

	unknown := w.send(config.WebhookPrefix+"/git/"+strings.Repeat("b", 32), giteaHeaders(secret, "big-2", big), big)
	requireError(t, unknown, http.StatusUnauthorized, "WEBHOOK_REJECTED")

	small := giteaPush("refs/heads/main", sha)
	chunked := w.env.WebhookFrom(apptest.WebhookRequest{
		Method: http.MethodPost, Path: hookPath(r), Headers: giteaHeaders(secret, "chunked-1", small), Body: small, Chunked: true,
	})
	requireError(t, chunked, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
	assert.Equal(t, http.StatusAccepted, w.send(hookPath(r), giteaHeaders(secret, "small-1", small), small).Status)
}

func TestChunkedBodiesStopAtTheLimit(t *testing.T) {
	w := newWorld(t, false)
	r, secret := linkHTTPS(t, w)
	huge := make([]byte, config.WebhookMaxBytesDefault+1)
	res := w.env.WebhookFrom(apptest.WebhookRequest{
		Method: http.MethodPost, Path: hookPath(r), Headers: giteaHeaders(secret, "chunked-big", huge), Body: huge, Chunked: true,
	})
	requireError(t, res, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE")
}
