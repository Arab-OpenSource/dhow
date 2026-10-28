package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

func TestRoleChangeRevokesTheMembersSessionsAndIsAudited(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	dev, devID := env.SignedIn(org, "dev@example.com", "developer")
	second, res := env.Login("dev@example.com", "a long password for dev@example.com")
	require.Equal(t, http.StatusOK, res.Status)

	changed := owner.ChangeRole(org, devID.String(), "maintainer", nil)
	require.Equal(t, http.StatusOK, changed.Status, string(changed.Raw))
	var change struct {
		UserId, Role string
		Revoked      int
	}
	changed.Decode(t, &change)
	assert.Equal(t, "maintainer", change.Role)
	assert.Equal(t, 2, change.Revoked)
	assert.False(t, env.SessionAlive(dev), "the member's sessions end at once")
	assert.False(t, env.SessionAlive(second))
	assert.True(t, env.SessionAlive(owner), "the actor stays signed in")

	again, _ := env.Login("dev@example.com", "a long password for dev@example.com")
	assert.Equal(t, "maintainer", again.RoleIn(org))
	assert.Equal(t, 1, env.AuditCount("identity.member.role_changed"))
}

func TestNobodyGrantsARoleAboveTheirOwn(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	admin, adminID := env.SignedIn(org, "admin@example.com", "admin")
	_, viewerID := env.SignedIn(org, "viewer@example.com", "viewer")
	maintainer, _ := env.SignedIn(org, "maint@example.com", "maintainer")
	var ownerID string
	require.NoError(t, env.Pool.QueryRow(t.Context(), `SELECT "Id"::text FROM "Users" WHERE "Email" = 'owner@example.com'`).Scan(&ownerID))

	tests := []struct {
		name   string
		actor  *apptest.Client
		target string
		role   string
		status int
		code   string
	}{
		{"admin cannot make anyone owner", admin, viewerID.String(), "owner", http.StatusForbidden, "ROLE_NOT_ALLOWED"},
		{"admin cannot demote the owner", admin, ownerID, "viewer", http.StatusForbidden, "ROLE_NOT_ALLOWED"},
		{"admin cannot change themselves", admin, adminID.String(), "owner", http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"owner cannot change themselves", owner, ownerID, "admin", http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"maintainer has no member.manage", maintainer, viewerID.String(), "developer", http.StatusForbidden, "FORBIDDEN"},
		{"an unknown role", owner, viewerID.String(), "root", http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"granting owner needs a factor", owner, viewerID.String(), "owner", http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"granting owner with a wrong factor", owner, viewerID.String(), "owner", http.StatusForbidden, "INVALID_CURRENT_FACTOR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var factor map[string]any
			if tt.name == "granting owner with a wrong factor" {
				factor = apptest.PasswordFactor("not the password")
			}
			res := tt.actor.ChangeRole(org, tt.target, tt.role, factor)
			assert.Equal(t, tt.status, res.Status, string(res.Raw))
			require.NotNil(t, res.Error)
			assert.Equal(t, tt.code, res.Error.Code)
		})
	}
	assert.Equal(t, "viewer", roleOf(t, env, org.String(), viewerID.String()))
	assert.Equal(t, "admin", roleOf(t, env, org.String(), adminID.String()))

	ok := admin.ChangeRole(org, viewerID.String(), "admin", nil)
	assert.Equal(t, http.StatusOK, ok.Status, "an admin may grant admin, which is their own role: %s", ok.Raw)
	promoted := owner.ChangeRole(org, viewerID.String(), "owner", apptest.PasswordFactor(d.Password))
	assert.Equal(t, http.StatusOK, promoted.Status, string(promoted.Raw))
	assert.Equal(t, "owner", roleOf(t, env, org.String(), viewerID.String()))
	assert.Equal(t, 1, highSeverity(t, env, "identity.member.role_changed"))
}

func TestLastOwnerIsProtected(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	me := owner.Me().User.Id

	leave := owner.Do(apptest.Request{Method: http.MethodPost, Path: apptest.OrgPath(org, "/leave")})
	assert.Equal(t, http.StatusConflict, leave.Status, string(leave.Raw))
	assert.Equal(t, "LAST_OWNER", leave.Error.Code)
	self := owner.Do(apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(org, "/members/"+me)})
	assert.Equal(t, http.StatusConflict, self.Status, "removing yourself as the last owner: %s", self.Raw)
	assert.Equal(t, "LAST_OWNER", self.Error.Code)
	assert.Equal(t, "owner", owner.RoleIn(org))

	coOwner, coID := env.SignedIn(org, "co@example.com", "owner")
	demote := coOwner.ChangeRole(org, me, "admin", nil)
	assert.Equal(t, http.StatusUnprocessableEntity, demote.Status, "taking away owner needs a factor")
	demote = coOwner.ChangeRole(org, me, "admin", apptest.PasswordFactor("a long password for co@example.com"))
	require.Equal(t, http.StatusOK, demote.Status, string(demote.Raw))
	again, _ := env.Login(d.Email, d.Password)
	gone := again.Do(apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(org, "/members/"+coID.String())})
	assert.Equal(t, http.StatusForbidden, gone.Status, "an admin cannot remove an owner")
	left := coOwner.Do(apptest.Request{Method: http.MethodPost, Path: apptest.OrgPath(org, "/leave")})
	assert.Equal(t, http.StatusConflict, left.Status, "the remaining owner is the last one")
}

func TestRemovingAMemberEndsTheirAccessAndTeams(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	dev, devID := env.SignedIn(org, "dev@example.com", "developer")
	team := createTeam(t, owner, org, "Backend", "")
	addToTeam(t, owner, org, team, devID.String())

	res := owner.Do(apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(org, "/members/"+devID.String())})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	assert.False(t, env.SessionAlive(dev))
	again, _ := env.Login("dev@example.com", "a long password for dev@example.com")
	assert.Equal(t, http.StatusNotFound, again.Get(apptest.OrgPath(org, "")).Status)
	var count int
	require.NoError(t, env.Pool.QueryRow(t.Context(), `SELECT count(*) FROM "TeamMembers" WHERE "UserId" = $1 AND NOT "IsDeleted"`, devID).Scan(&count))
	assert.Zero(t, count)
	assert.Equal(t, 1, env.AuditCount("identity.member.removed"))
}

func TestTransferOwnershipNeedsAFactor(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	admin, adminID := env.SignedIn(org, "admin@example.com", "admin")
	transfer := func(c *apptest.Client, factor map[string]any) apptest.Response {
		return c.Do(apptest.Request{
			Method: http.MethodPost, Path: apptest.OrgPath(org, "/transfer-ownership"),
			Body: map[string]any{"UserId": adminID.String(), "CurrentFactor": factor},
		})
	}
	assert.Equal(t, http.StatusForbidden, transfer(admin, apptest.PasswordFactor("a long password for admin@example.com")).Status, "only owners transfer")
	wrong := transfer(owner, apptest.PasswordFactor("not the password"))
	assert.Equal(t, "INVALID_CURRENT_FACTOR", wrong.Error.Code)
	self := owner.Do(apptest.Request{
		Method: http.MethodPost, Path: apptest.OrgPath(org, "/transfer-ownership"),
		Body: map[string]any{"UserId": owner.Me().User.Id, "CurrentFactor": apptest.PasswordFactor(d.Password)},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, self.Status)

	ok := transfer(owner, apptest.PasswordFactor(d.Password))
	require.Equal(t, http.StatusOK, ok.Status, string(ok.Raw))
	assert.False(t, env.SessionAlive(admin), "the new owner signs in again")
	assert.True(t, env.SessionAlive(owner), "the old owner keeps the current session")
	assert.Equal(t, "admin", owner.RoleIn(org))
	assert.Equal(t, "owner", roleOf(t, env, org.String(), adminID.String()))
	assert.Equal(t, 1, highSeverity(t, env, "identity.organization.ownership_transferred"))
}

func roleOf(t *testing.T, env *apptest.Env, org, user string) string {
	t.Helper()
	var role string
	require.NoError(t, env.Pool.QueryRow(t.Context(),
		`SELECT "Role" FROM "Memberships" WHERE "OrganizationId" = $1 AND "UserId" = $2 AND NOT "IsDeleted"`, org, user).Scan(&role))
	return role
}

func highSeverity(t *testing.T, env *apptest.Env, action string) int {
	t.Helper()
	var n int
	require.NoError(t, env.Pool.QueryRow(t.Context(),
		`SELECT count(*) FROM "AuditEvents" WHERE "Action" = $1 AND "Data"->>'Severity' = 'high'`, action).Scan(&n))
	return n
}
