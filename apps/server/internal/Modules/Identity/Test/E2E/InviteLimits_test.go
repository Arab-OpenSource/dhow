package e2e_test

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
)

func limitedInvites(t *testing.T, perUser, perOrg int) (*apptest.Env, *apptest.Client, uuid.UUID) {
	t.Helper()
	env := apptest.NewWithOptions(t, apptest.Options{Mailer: &apptest.FakeMailer{}, Profile: func(c *config.Config) {
		c.RateLimits.InvitesUser, c.RateLimits.InvitesOrg = perUser, perOrg
	}})
	owner, d := env.SetupAndLogin()
	return env, owner, env.OrganizationID(d.OrgSlug)
}

func resend(c *apptest.Client, org uuid.UUID, id string) apptest.Response {
	return c.Do(apptest.Request{Method: http.MethodPost, Path: apptest.OrgPath(org, "/invitations/"+id+"/resend")})
}

func requireRateLimited(t *testing.T, res apptest.Response) {
	t.Helper()
	require.Equal(t, http.StatusTooManyRequests, res.Status, string(res.Raw))
	assert.Equal(t, apperrors.CodeRateLimited, res.Error.Code)
	retry, err := strconv.Atoi(res.Header.Get("Retry-After"))
	require.NoError(t, err)
	assert.Positive(t, retry)
	assert.LessOrEqual(t, retry, int(config.RateLimitInviteWindow.Seconds()))
}

func TestOneMemberCanSendOnlySoManyInvitationsAnHour(t *testing.T) {
	env, owner, org := limitedInvites(t, 3, 100)
	first := owner.MustInvite(org, "a@example.com", "developer")
	owner.MustInvite(org, "b@example.com", "developer")
	require.Equal(t, http.StatusOK, resend(owner, org, first.Invitation.Id).Status, "a resend counts too")
	requireRateLimited(t, owner.Invite(org, "c@example.com", "developer").Response)
	requireRateLimited(t, resend(owner, org, first.Invitation.Id))

	var open int
	require.NoError(t, env.Pool.QueryRow(t.Context(), `SELECT count(*) FROM "Invitations" WHERE "Email" = 'c@example.com'`).Scan(&open))
	assert.Zero(t, open, "a limited request creates nothing")

	admin, _ := env.SignedIn(org, "admin@example.com", "admin")
	admin.MustInvite(org, "c@example.com", "developer")

	env.Clock.Advance(config.RateLimitInviteWindow)
	owner.MustInvite(org, "d@example.com", "developer")
}

func TestOneOrganizationCanReceiveOnlySoManyInvitationsAnHour(t *testing.T) {
	env, owner, org := limitedInvites(t, 100, 2)
	admin, _ := env.SignedIn(org, "admin@example.com", "admin")
	owner.MustInvite(org, "a@example.com", "developer")
	admin.MustInvite(org, "b@example.com", "developer")
	requireRateLimited(t, owner.Invite(org, "c@example.com", "developer").Response)
	requireRateLimited(t, admin.Invite(org, "c@example.com", "developer").Response)

	other := owner.Do(apptest.Request{Method: http.MethodPost, Path: "/organizations", Body: map[string]string{"Name": "Other", "Slug": "other"}})
	require.Equal(t, http.StatusCreated, other.Status, string(other.Raw))
	var created struct{ Id uuid.UUID }
	other.Decode(t, &created)
	owner.MustInvite(created.Id, "c@example.com", "developer")
}

func TestAnInvitationGetsAtMostFiveSends(t *testing.T) {
	env, owner, org := limitedInvites(t, 100, 100)
	issued := owner.MustInvite(org, "a@example.com", "developer")
	for i := 2; i <= config.InvitationMaxSends; i++ {
		res := resend(owner, org, issued.Invitation.Id)
		require.Equal(t, http.StatusOK, res.Status, "send %d: %s", i, res.Raw)
	}
	res := resend(owner, org, issued.Invitation.Id)
	require.Equal(t, http.StatusConflict, res.Status, string(res.Raw))
	assert.Equal(t, apperrors.CodeInvitationSendLimit, res.Error.Code)

	var sends int
	require.NoError(t, env.Pool.QueryRow(t.Context(), `SELECT "SendCount" FROM "Invitations" WHERE "Id" = $1`, issued.Invitation.Id).Scan(&sends))
	assert.Equal(t, config.InvitationMaxSends, sends)

	revoked := owner.Do(apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(org, "/invitations/"+issued.Invitation.Id)})
	require.Equal(t, http.StatusOK, revoked.Status, string(revoked.Raw))
	owner.MustInvite(org, "a@example.com", "developer")
}

func TestTheInvitationLimitsDoNotCountForbiddenRequests(t *testing.T) {
	env, owner, org := limitedInvites(t, 2, 2)
	viewer, _ := env.SignedIn(org, "viewer@example.com", "viewer")
	for i := range 5 {
		res := viewer.Invite(org, fmt.Sprintf("v%d@example.com", i), "viewer")
		require.Equal(t, http.StatusForbidden, res.Response.Status, string(res.Response.Raw))
	}
	owner.MustInvite(org, "a@example.com", "developer")
	owner.MustInvite(org, "b@example.com", "developer")
}

func TestAResendThatCannotHappenSpendsNothing(t *testing.T) {
	env, owner, org := limitedInvites(t, 7, 100)
	issued := owner.MustInvite(org, "a@example.com", "developer")
	for i := 2; i <= config.InvitationMaxSends; i++ {
		require.Equal(t, http.StatusOK, resend(owner, org, issued.Invitation.Id).Status)
	}
	for range 5 {
		require.Equal(t, http.StatusNotFound, resend(owner, org, uuid.NewString()).Status, "an unknown invitation")
		require.Equal(t, http.StatusConflict, resend(owner, org, issued.Invitation.Id).Status, "an invitation at its send limit")
	}
	admin, _ := env.SignedIn(org, "admin@example.com", "admin")
	for range 3 {
		require.Equal(t, http.StatusNotFound, resend(admin, org, uuid.NewString()).Status)
	}
	owner.MustInvite(org, "b@example.com", "developer")
	owner.MustInvite(org, "c@example.com", "developer")
	requireRateLimited(t, owner.Invite(org, "d@example.com", "developer").Response)
}
