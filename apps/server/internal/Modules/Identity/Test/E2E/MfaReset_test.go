package e2e_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

func resetMfa(c *apptest.Client, org uuid.UUID, user string, factor map[string]any) apptest.Response {
	return c.Do(apptest.Request{
		Method: http.MethodPost, Path: apptest.OrgPath(org, "/members/"+user+"/mfa-reset"),
		Body: map[string]any{"CurrentFactor": factor},
	})
}

func TestAdminResetsALostSecondFactor(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	ownerTotp := owner.EnableTotp(apptest.PasswordFactor(d.Password))

	const devPassword = "a long password for dev@example.com"
	dev, devID := env.SignedIn(org, "dev@example.com", "developer")
	dev.EnableTotp(apptest.PasswordFactor(devPassword))
	assert.Equal(t, http.StatusAccepted, env.PasswordStep("dev@example.com", devPassword, nil).Status, "the member has a second step")
	require.True(t, env.SessionAlive(dev))

	password := resetMfa(owner, org, devID.String(), apptest.PasswordFactor(d.Password))
	assert.Equal(t, "INVALID_CURRENT_FACTOR", password.Error.Code, "the password alone is not the actor's current factor once they have TOTP")
	self := resetMfa(owner, org, owner.Me().User.Id, apptest.CodeFactor("totp", env.TotpCode(ownerTotp.Secret, 1)))
	assert.Equal(t, http.StatusUnprocessableEntity, self.Status, "your own factors change in your account")

	ok := resetMfa(owner, org, devID.String(), apptest.CodeFactor("totp", env.TotpCode(ownerTotp.Secret, 1)))
	require.Equal(t, http.StatusOK, ok.Status, string(ok.Raw))
	assert.False(t, env.SessionAlive(dev), "the member is signed out everywhere")
	assert.Equal(t, 1, highSeverity(t, env, "identity.member.mfa_reset"))

	again, res := env.Login("dev@example.com", devPassword)
	require.Equal(t, http.StatusOK, res.Status, "the password alone signs in again: %s", res.Raw)
	assert.True(t, again.Me().MfaReenrollRequired)
	var members []struct {
		Email           string
		Mfa             string
		MfaResetPending bool
	}
	owner.Get(apptest.OrgPath(org, "/members")).Decode(t, &members)
	for _, m := range members {
		if m.Email == "dev@example.com" {
			assert.Equal(t, "none", m.Mfa)
			assert.True(t, m.MfaResetPending)
		}
	}
	again.EnableTotp(apptest.PasswordFactor(devPassword))
	assert.False(t, again.Me().MfaReenrollRequired, "a new factor clears the flag")
	env.Clock.Advance(time.Minute)
	nothing := resetMfa(owner, org, devID.String(), apptest.CodeFactor("totp", env.TotpCode(ownerTotp.Secret, 0)))
	assert.Equal(t, http.StatusOK, nothing.Status, "a second reset works too: %s", nothing.Raw)
}

func TestMfaResetNeedsAuthorityOverEveryOrganizationOfTheMember(t *testing.T) {
	env := apptest.New(t)
	_, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	admin, _ := env.SignedIn(org, "admin@example.com", "admin")
	const password = "a long password for shared@example.com"
	_, sharedID := env.SignedIn(org, "shared@example.com", "viewer")
	other := env.SeedOrganization("Other", "other")
	_, err := env.Pool.Exec(t.Context(), `INSERT INTO "Memberships" ("UserId", "OrganizationId", "Role") VALUES ($1, $2, 'owner')`, sharedID, other)
	require.NoError(t, err)
	shared, _ := env.Login("shared@example.com", password)
	shared.EnableTotp(apptest.PasswordFactor(password))

	res := resetMfa(admin, org, sharedID.String(), apptest.PasswordFactor("a long password for admin@example.com"))
	assert.Equal(t, http.StatusForbidden, res.Status, "the member owns an organization the admin does not manage: %s", res.Raw)
	assert.Equal(t, http.StatusAccepted, env.PasswordStep("shared@example.com", password, nil).Status, "nothing was reset")

	owner, _ := env.SignedIn(org, "owner2@example.com", "owner")
	onlyOwner := resetMfa(owner, org, sharedID.String(), apptest.PasswordFactor("a long password for owner2@example.com"))
	assert.Equal(t, http.StatusForbidden, onlyOwner.Status, "owning this organization is not enough either")

	none := resetMfa(admin, org, admin.Me().User.Id, apptest.PasswordFactor("x"))
	assert.Equal(t, http.StatusUnprocessableEntity, none.Status)
	stranger := resetMfa(admin, org, uuid.NewString(), apptest.PasswordFactor("a long password for admin@example.com"))
	assert.Equal(t, http.StatusNotFound, stranger.Status)
}

func TestAnAdminCannotResetAnOwner(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	owner.EnableTotp(apptest.PasswordFactor(d.Password))
	admin, _ := env.SignedIn(org, "admin@example.com", "admin")
	var ownerID string
	require.NoError(t, env.Pool.QueryRow(t.Context(), `SELECT "Id"::text FROM "Users" WHERE "Email" = 'owner@example.com'`).Scan(&ownerID))
	res := resetMfa(admin, org, ownerID, apptest.PasswordFactor("a long password for admin@example.com"))
	assert.Equal(t, http.StatusForbidden, res.Status, string(res.Raw))
	assert.Equal(t, http.StatusAccepted, env.PasswordStep(d.Email, d.Password, nil).Status)
}

func TestMfaResetRevokesTheMemberAPITokens(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	ownerToken := owner.MustCreateAPIToken(org, []string{"member.read"}, d.Password)

	const devPassword = "a long password for dev@example.com"
	dev, devID := env.SignedIn(org, "dev@example.com", "developer")
	devToken := dev.MustCreateAPIToken(org, []string{"member.read"}, devPassword)
	dev.EnableTotp(apptest.PasswordFactor(devPassword))
	members := apptest.Request{Method: http.MethodGet, Path: apptest.OrgPath(org, "/members")}
	require.Equal(t, http.StatusOK, env.Bearer(devToken.Value, members).Status)

	res := resetMfa(owner, org, devID.String(), apptest.PasswordFactor(d.Password))
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	assert.Equal(t, http.StatusUnauthorized, env.Bearer(devToken.Value, members).Status, "a reset is a compromise recovery, so the member's tokens stop")
	assert.Equal(t, http.StatusOK, env.Bearer(ownerToken.Value, members).Status, "other members' tokens keep working")
	var revoked string
	require.NoError(t, env.Pool.QueryRow(t.Context(),
		`SELECT "Data"->>'RevokedPats' FROM "AuditEvents" WHERE "Action" = 'identity.member.mfa_reset'`).Scan(&revoked))
	assert.Equal(t, "1", revoked)
}
