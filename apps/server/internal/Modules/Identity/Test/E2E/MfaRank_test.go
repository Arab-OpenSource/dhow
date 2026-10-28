package e2e_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
)

func passwordOf(email string) map[string]any {
	return apptest.PasswordFactor("a long password for " + email)
}

func withTotp(t *testing.T, env *apptest.Env, org uuid.UUID, email, role string) uuid.UUID {
	t.Helper()
	c, id := env.SignedIn(org, email, role)
	c.EnableTotp(passwordOf(email))
	return id
}

func setUserFlag(t *testing.T, env *apptest.Env, column string, user uuid.UUID) {
	t.Helper()
	_, err := env.Pool.Exec(t.Context(), `UPDATE "Users" SET "`+column+`" = true WHERE "Id" = $1`, user)
	require.NoError(t, err)
}

func TestMfaResetNeedsAStrictlyHigherEffectiveRole(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	ownerFactor := apptest.PasswordFactor(d.Password)
	admin, _ := env.SignedIn(org, "admin@example.com", "admin")
	adminFactor := passwordOf("admin@example.com")

	peer := withTotp(t, env, org, "peer@example.com", "admin")
	res := resetMfa(admin, org, peer.String(), adminFactor)
	assert.Equal(t, http.StatusForbidden, res.Status, "an admin never resets another admin: %s", res.Raw)

	raised := withTotp(t, env, org, "raised@example.com", "viewer")
	team := createTeam(t, owner, org, "Admins", "admin")
	require.Equal(t, http.StatusOK, addToTeam(t, owner, org, team, raised.String()).Status)
	res = resetMfa(admin, org, raised.String(), adminFactor)
	assert.Equal(t, http.StatusForbidden, res.Status, "the target's effective role counts, not only the direct one: %s", res.Raw)

	res = resetMfa(admin, org, owner.Me().User.Id, adminFactor)
	assert.Equal(t, http.StatusForbidden, res.Status, "an admin never resets an owner: %s", res.Raw)

	maintainer := withTotp(t, env, org, "maintainer@example.com", "maintainer")
	res = resetMfa(admin, org, maintainer.String(), adminFactor)
	assert.Equal(t, http.StatusOK, res.Status, "an admin resets a maintainer: %s", res.Raw)

	res = resetMfa(owner, org, peer.String(), ownerFactor)
	assert.Equal(t, http.StatusOK, res.Status, "an owner resets an admin: %s", res.Raw)

	second := withTotp(t, env, org, "second@example.com", "owner")
	res = resetMfa(owner, org, second.String(), ownerFactor)
	assert.Equal(t, http.StatusForbidden, res.Status, "owners never reset each other: %s", res.Raw)
}

func TestTheInstanceOwnerCountsAsAnOwnerForMfaReset(t *testing.T) {
	env := apptest.New(t)
	_, d := env.SetupAndLogin()
	other := env.SeedOrganization("Other", "other")
	boss, _ := env.SignedIn(other, "boss@example.com", "owner")
	viewer := withTotp(t, env, other, "viewer@example.com", "viewer")
	setUserFlag(t, env, "IsInstanceOwner", viewer)

	res := resetMfa(boss, other, viewer.String(), passwordOf("boss@example.com"))
	assert.Equal(t, http.StatusForbidden, res.Status, "the instance owner is owner-level everywhere: %s", res.Raw)
	assert.NotEmpty(t, d.OrgSlug)
}
