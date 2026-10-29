package e2e_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

func TestAnOverdueAdminDoesNotSeeTheBlocksTheSecondFactorGuards(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	enrollment := owner.EnableTotp(apptest.PasswordFactor(d.Password))
	env.Clock.Advance(30 * time.Second)
	fresh := apptest.CodeFactor("totp", env.TotpCode(enrollment.Secret, 0))
	_, adminID := env.SignedIn(org, "admin@example.com", "admin")
	require.Equal(t, http.StatusOK, owner.ChangeSecondFactorRule(org, "admins", fresh).Status)
	owner.MustInvite(org, "pending@example.com", "viewer")

	overviewOf := func(id uuid.UUID) map[string]json.RawMessage {
		res := env.SeedSession(id).Get(apptest.OrgPath(org, "/overview"))
		require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
		var raw map[string]json.RawMessage
		res.Decode(t, &raw)
		return raw
	}

	inside := overviewOf(adminID)
	assert.JSONEq(t, `{"Pending":1}`, string(inside["Invitations"]), "the grace period allows everything")

	env.Clock.Advance(config.SecondFactorGrace)
	overdue := overviewOf(adminID)
	assert.JSONEq(t, "null", string(overdue["Invitations"]), "member.manage is sensitive, so its count is held back like the list")
	assert.NotEqual(t, "null", string(overdue["Jobs"]), "reading jobs stays allowed")
	assert.NotEqual(t, "null", string(overdue["Runners"]), "reading runners stays allowed")

	assert.NotEqual(t, "null", string(overviewOf(uuid.MustParse(owner.UserID))["Invitations"]), "the owner has TOTP")
}
