package e2e_test

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

var webhookURL = regexp.MustCompile(`^` + regexp.QuoteMeta(apptest.PublicOrigin) + `/webhooks/git/[0-9a-f]{32}$`)

func auditData(t *testing.T, w world) string {
	t.Helper()
	rows, err := w.env.Pool.Query(context.Background(), `SELECT "Action" || ' ' || COALESCE("Data"::text, '') FROM "AuditEvents"`)
	require.NoError(t, err)
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		b.WriteString(line + "\n")
	}
	require.NoError(t, rows.Err())
	return b.String()
}

func TestLinkAnHTTPSRepositoryShowsTheWebhookSecretOnce(t *testing.T) {
	w := newWorld(t, false)
	requireError(t, w.owner.Get(w.path("")), http.StatusNotFound, "REPOSITORY_NOT_LINKED")

	first := linked(t, w.link(t, w.owner, "https://Git.Example.com/team/app.git"), http.StatusCreated)
	require.NotNil(t, first.WebhookSecret)
	secret := *first.WebhookSecret
	assert.Len(t, secret, 43)
	r := first.Repository
	assert.Equal(t, "https://git.example.com/team/app.git", r.CloneUrl)
	assert.Equal(t, "generic", r.Provider)
	assert.Equal(t, "https", r.Transport)
	assert.Equal(t, "main", r.DefaultBranch)
	assert.Equal(t, "ready", r.State)
	assert.Nil(t, r.DeployKeyPublic)
	assert.Regexp(t, webhookURL, r.WebhookUrl)
	assert.NotContains(t, r.WebhookUrl, r.Id)

	reads := []apptest.Response{
		w.owner.Get(w.path("")),
		w.link(t, w.owner, "https://git.example.com/team/app.git"),
		w.owner.Do(apptest.Request{Method: http.MethodPut, Path: w.path(""), Body: map[string]any{"CloneUrl": "https://git.example.com/team/web.git", "DefaultBranch": "trunk"}}),
	}
	for _, res := range reads {
		assert.Less(t, res.Status, 300, string(res.Raw))
		assert.NotContains(t, string(res.Raw), secret)
		assert.NotContains(t, string(res.Raw), "WebhookSecret")
	}
	again := linked(t, reads[2], http.StatusOK)
	assert.Equal(t, "trunk", again.Repository.DefaultBranch)
	assert.Equal(t, r.WebhookUrl, again.Repository.WebhookUrl)

	rotated := linked(t, w.post(w.owner, "rotate-webhook-secret", nil), http.StatusOK)
	require.NotNil(t, rotated.WebhookSecret)
	assert.NotEqual(t, secret, *rotated.WebhookSecret)
	after := w.owner.Get(w.path(""))
	assert.NotContains(t, string(after.Raw), *rotated.WebhookSecret)

	audit := auditData(t, w)
	assert.Contains(t, audit, "project.repository_linked")
	assert.Contains(t, audit, "project.repository_updated")
	assert.Contains(t, audit, "project.webhook_secret_rotated")
	for _, s := range []string{secret, *rotated.WebhookSecret} {
		assert.NotContains(t, audit, s)
		assert.NotContains(t, w.env.Logs.String(), s)
	}
	requireError(t, w.post(w.owner, "rotate-deploy-key", nil), http.StatusConflict, "DEPLOY_KEY_NOT_APPLICABLE")
}

func TestLinkAnSSHRepositoryPinsTheHostKeyOnlyAfterConfirmation(t *testing.T) {
	w := newWorld(t, true)
	srv := startSSH(t)
	requireError(t, w.link(t, w.owner, "git@git.example.test:team/app.git"), http.StatusBadGateway, "GIT_HOST_UNREACHABLE")

	address := "ssh://git@git.example.test:" + srv.port + "/team/app.git"
	l := linked(t, w.link(t, w.owner, address), http.StatusCreated)
	r := l.Repository
	assert.Equal(t, "ssh", r.Transport)
	assert.Equal(t, "host_key_unconfirmed", r.State)
	require.NotNil(t, r.DeployKeyPublic)
	assert.True(t, strings.HasPrefix(*r.DeployKeyPublic, "ssh-ed25519 "), *r.DeployKeyPublic)
	require.NotNil(t, r.PendingHostKeyFingerprint)
	assert.Equal(t, srv.fingerprint, *r.PendingHostKeyFingerprint)
	assert.Nil(t, r.HostKeyFingerprint)

	requireError(t, w.post(w.owner, "confirm-host-key", map[string]string{"Fingerprint": "SHA256:" + strings.Repeat("A", 43)}),
		http.StatusConflict, "HOST_KEY_FINGERPRINT_MISMATCH")
	pinned := repo(t, w.post(w.owner, "confirm-host-key", map[string]string{"Fingerprint": srv.fingerprint}))
	assert.Equal(t, "ready", pinned.State)
	require.NotNil(t, pinned.HostKeyFingerprint)
	assert.Equal(t, srv.fingerprint, *pinned.HostKeyFingerprint)
	assert.Nil(t, pinned.PendingHostKeyFingerprint)
	requireError(t, w.post(w.owner, "confirm-host-key", map[string]string{"Fingerprint": srv.fingerprint}),
		http.StatusConflict, "HOST_KEY_NOT_PENDING")

	same := linked(t, w.link(t, w.owner, address), http.StatusOK)
	assert.Equal(t, srv.fingerprint, *same.Repository.HostKeyFingerprint)
	assert.Nil(t, same.Repository.PendingHostKeyFingerprint)
	assert.Equal(t, *r.DeployKeyPublic, *same.Repository.DeployKeyPublic)

	_, err := w.env.Pool.Exec(context.Background(), `UPDATE "Repositories" SET "HostKey" = 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl' WHERE "Id" = $1`, r.Id)
	require.NoError(t, err)
	changed := linked(t, w.link(t, w.owner, address), http.StatusOK).Repository
	assert.Equal(t, "ready", changed.State, "a changed host key stays pending; the pinned one is never replaced by itself")
	require.NotNil(t, changed.PendingHostKeyFingerprint)
	assert.Equal(t, srv.fingerprint, *changed.PendingHostKeyFingerprint)
	assert.NotEqual(t, srv.fingerprint, *changed.HostKeyFingerprint)

	other := startSSH(t)
	moved := linked(t, w.link(t, w.owner, "ssh://git@git.example.test:"+other.port+"/team/app.git"), http.StatusOK).Repository
	assert.Equal(t, "host_key_unconfirmed", moved.State, "a new host or port drops the pinned key")
	assert.Nil(t, moved.HostKeyFingerprint)
	assert.Equal(t, other.fingerprint, *moved.PendingHostKeyFingerprint)

	var sealed []byte
	require.NoError(t, w.env.Pool.QueryRow(context.Background(), `SELECT "DeployKeySealed" FROM "Repositories" WHERE "Id" = $1`, r.Id).Scan(&sealed))
	assert.NotContains(t, string(sealed), "PRIVATE KEY")
	assert.NotContains(t, string(sealed), "openssh-key-v1")
	for _, res := range []apptest.Response{w.owner.Get(w.path("")), w.link(t, w.owner, address)} {
		assert.NotContains(t, string(res.Raw), "PRIVATE KEY")
	}
	assert.NotContains(t, w.env.Logs.String(), "PRIVATE KEY")
	assert.NotContains(t, auditData(t, w), "PRIVATE KEY")
}

func TestRotateTheDeployKeyKeepsThePreviousOneForADay(t *testing.T) {
	w := newWorld(t, true)
	srv := startSSH(t)
	r := linked(t, w.link(t, w.owner, "ssh://git@git.example.test:"+srv.port+"/app.git"), http.StatusCreated).Repository
	rotated := repo(t, w.post(w.owner, "rotate-deploy-key", nil))
	require.NotNil(t, rotated.PreviousDeployKeyPublic)
	assert.Equal(t, *r.DeployKeyPublic, *rotated.PreviousDeployKeyPublic)
	assert.NotEqual(t, *r.DeployKeyPublic, *rotated.DeployKeyPublic)

	confirmed := repo(t, w.post(w.owner, "confirm-deploy-key", nil))
	assert.Nil(t, confirmed.PreviousDeployKeyPublic)
	repo(t, w.post(w.owner, "confirm-deploy-key", nil))

	repo(t, w.post(w.owner, "rotate-deploy-key", nil))
	w.env.Clock.Advance(25 * time.Hour)
	res, err := w.env.Wired.Sources.Maintain(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), res.ExpiredKeys)
	owner := w.env.SeedSession(uuid.MustParse(w.owner.UserID))
	assert.Nil(t, repo(t, owner.Get(w.path(""))).PreviousDeployKeyPublic)
	assert.Contains(t, auditData(t, w), "project.deploy_key_rotated")
}

func TestLinkRefusesPrivateAndMetadataAddresses(t *testing.T) {
	strict := newWorld(t, false)
	srv := startSSH(t)
	for _, host := range []string{"git.example.test", "mixed.example.test", "mapped.example.test", "meta.example.test"} {
		t.Run("strict/"+host, func(t *testing.T) {
			requireError(t, strict.link(t, strict.owner, "ssh://git@"+host+":"+srv.port+"/app.git"), http.StatusUnprocessableEntity, "GIT_HOST_BLOCKED")
		})
	}

	open := newWorld(t, true)
	for _, host := range []string{"meta.example.test", "private.example.test"} {
		t.Run(host, func(t *testing.T) {
			requireError(t, open.link(t, open.owner, "ssh://git@"+host+":"+srv.port+"/app.git"), http.StatusUnprocessableEntity, "GIT_HOST_BLOCKED")
		})
	}
	requireError(t, open.link(t, open.owner, "ssh://git@missing.example.test/app.git"), http.StatusBadGateway, "GIT_HOST_UNREACHABLE")
	assert.Zero(t, open.env.CountRows(`SELECT count(*) FROM "Repositories"`))
}

func TestLinkRefusesDangerousAddressesAndBranches(t *testing.T) {
	w := newWorld(t, false)
	for _, address := range []string{
		"ext::sh -c touch% /tmp/pwned", "file:///etc/passwd", "-oProxyCommand=touch /tmp/x", "--upload-pack=touch",
		"ssh://git@git.example.test/-oProxyCommand=x", "https://git.example.com/a\nb", "https://git.example.com/a/../b",
		"https://user:pass@git.example.com/a", "https://127.0.0.1/a", "http://git.example.com/a",
	} {
		res := w.link(t, w.owner, address)
		requireError(t, res, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
		assert.Equal(t, "CloneUrl", res.Error.Details[0].Field, address)
	}
	for _, branch := range []string{"a..b", "main@{1}", "-x", "a b", "a\x00", "main.lock"} {
		res := w.owner.Do(apptest.Request{Method: http.MethodPut, Path: w.path(""), Body: map[string]any{"CloneUrl": "https://git.example.com/a.git", "DefaultBranch": branch}})
		requireError(t, res, http.StatusUnprocessableEntity, "VALIDATION_FAILED")
		assert.Equal(t, "DefaultBranch", res.Error.Details[0].Field, branch)
	}
	assert.Zero(t, w.env.CountRows(`SELECT count(*) FROM "Repositories"`))
}

func TestRepositoryRoutesFollowTheProjectRole(t *testing.T) {
	w := newWorld(t, false)
	linked(t, w.link(t, w.owner, "https://git.example.com/a.git"), http.StatusCreated)
	developer := w.env.SeedSession(w.env.SeedUser(w.org, uuid.NewString()[:8]+"@example.com", "a long password for a developer", "developer"))
	viewer := w.env.SeedSession(w.env.SeedUser(w.org, uuid.NewString()[:8]+"@example.com", "a long password for a viewer", "viewer"))
	repo(t, viewer.Get(w.path("")))
	requireError(t, developer.Do(apptest.Request{Method: http.MethodPut, Path: w.path(""), Body: map[string]any{"CloneUrl": "https://git.example.com/b.git"}}),
		http.StatusForbidden, "FORBIDDEN")
	requireError(t, viewer.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/rotate-webhook-secret")}), http.StatusForbidden, "FORBIDDEN")

	other := w.env.SeedOrganization("Other", "other")
	outsider := w.env.SeedSession(w.env.SeedUser(other, "o@example.com", "a long password for outsider", "owner"))
	requireError(t, outsider.Get(w.path("")), http.StatusNotFound, "NOT_FOUND")

	_, err := w.env.Pool.Exec(context.Background(), `UPDATE "Projects" SET "State" = 'archived', "ArchivedAt" = now() WHERE "Id" = $1`, w.project)
	require.NoError(t, err)
	requireError(t, w.link(t, w.owner, "https://git.example.com/c.git"), http.StatusConflict, "PROJECT_ARCHIVED")
	requireError(t, w.post(w.owner, "rotate-webhook-secret", nil), http.StatusConflict, "PROJECT_ARCHIVED")
	repo(t, w.owner.Get(w.path("")))
}
