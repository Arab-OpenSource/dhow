package e2e_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
)

func rename(c *apptest.Client, org uuid.UUID, name string) apptest.Response {
	return c.Do(apptest.Request{Method: http.MethodPatch, Path: apptest.OrgPath(org, ""), Body: map[string]string{"Name": name}})
}

func errorCode(res apptest.Response) string {
	if res.Error == nil {
		return ""
	}
	return res.Error.Code
}

type ruleView struct {
	RequireSecondFactor      string
	RequireSecondFactorSince *time.Time
}

func ruleOf(t *testing.T, c *apptest.Client, org uuid.UUID) ruleView {
	t.Helper()
	res := c.Get(apptest.OrgPath(org, ""))
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var v ruleView
	res.Decode(t, &v)
	return v
}

const totpStep = 30 * time.Second

func ownerWithTotp(t *testing.T, env *apptest.Env) (*apptest.Client, uuid.UUID, func() map[string]any) {
	t.Helper()
	owner, d := env.SetupAndLogin()
	enrollment := owner.EnableTotp(apptest.PasswordFactor(d.Password))
	fresh := func() map[string]any {
		env.Clock.Advance(totpStep)
		return apptest.CodeFactor("totp", env.TotpCode(enrollment.Secret, 0))
	}
	return owner, env.OrganizationID(d.OrgSlug), fresh
}

func TestAnOwnerWithoutASecondFactorCannotRequireOne(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)

	res := owner.ChangeSecondFactorRule(org, "admins", apptest.PasswordFactor(d.Password))
	require.Equal(t, http.StatusForbidden, res.Status, string(res.Raw))
	assert.Equal(t, "SECOND_FACTOR_REQUIRED", res.Error.Code)
	assert.NotEmpty(t, res.Error.Message)
	assert.Equal(t, "off", ruleOf(t, owner, org).RequireSecondFactor)
	assert.Zero(t, env.AuditCount("identity.organization.second_factor_changed"))
}

func TestOnlyAnOwnerWithTheCurrentFactorChangesTheRule(t *testing.T) {
	env := apptest.New(t)
	owner, org, fresh := ownerWithTotp(t, env)
	admin, adminID := env.SignedIn(org, "admin@example.com", "admin")
	env.SeedPasskey(adminID)

	denied := admin.ChangeSecondFactorRule(org, "admins", apptest.PasswordFactor("a long password for admin@example.com"))
	assert.Equal(t, http.StatusForbidden, denied.Status, string(denied.Raw))
	assert.Equal(t, "FORBIDDEN", errorCode(denied))

	wrong := owner.ChangeSecondFactorRule(org, "admins", apptest.CodeFactor("totp", "000000"))
	assert.Equal(t, http.StatusForbidden, wrong.Status, string(wrong.Raw))
	assert.Equal(t, "INVALID_CURRENT_FACTOR", errorCode(wrong))

	invalid := owner.ChangeSecondFactorRule(org, "sometimes", fresh())
	assert.Equal(t, http.StatusUnprocessableEntity, invalid.Status, string(invalid.Raw))
	assert.Equal(t, "VALIDATION_FAILED", errorCode(invalid))

	ok := owner.ChangeSecondFactorRule(org, "admins", fresh())
	require.Equal(t, http.StatusOK, ok.Status, string(ok.Raw))
	view := ruleOf(t, owner, org)
	assert.Equal(t, "admins", view.RequireSecondFactor)
	require.NotNil(t, view.RequireSecondFactorSince)
	assert.WithinDuration(t, env.Clock.Now(), *view.RequireSecondFactorSince, time.Second)
	assert.Equal(t, 1, env.AuditCount("identity.organization.second_factor_changed"))

	token := env.SeedAPIToken(uuid.MustParse(owner.UserID), org, "org.update")
	viaToken := env.Bearer(token.Value, apptest.Request{Method: http.MethodPut, Path: apptest.OrgPath(org, "/second-factor-rule"), Body: map[string]any{
		"RequireSecondFactor": "off", "CurrentFactor": fresh(),
	}})
	assert.Equal(t, "SESSION_REQUIRED", errorCode(viaToken), string(viaToken.Raw))
	assert.Equal(t, "admins", ruleOf(t, owner, org).RequireSecondFactor)
}

func TestTheGracePeriodThenTheBlock(t *testing.T) {
	env := apptest.New(t)
	owner, org, fresh := ownerWithTotp(t, env)
	admin, adminID := env.SignedIn(org, "admin@example.com", "admin")
	developer, _ := env.SignedIn(org, "dev@example.com", "developer")
	require.Equal(t, http.StatusOK, owner.ChangeSecondFactorRule(org, "admins", fresh()).Status)
	enabled := env.Clock.Now()
	ownerID := uuid.MustParse(owner.UserID)

	due := admin.SecondFactorDue(org)
	require.NotNil(t, due, "a covered member without a factor sees the deadline")
	assert.WithinDuration(t, enabled.Add(config.SecondFactorGrace), *due, time.Second)
	assert.Nil(t, developer.SecondFactorDue(org), "admins does not cover a developer")
	assert.Nil(t, owner.SecondFactorDue(org), "an owner with TOTP owes nothing")

	env.Clock.Advance(config.SecondFactorGrace - time.Minute)
	admin = env.SeedSession(adminID)
	assert.Equal(t, http.StatusOK, rename(admin, org, "Inside the grace period").Status, "the grace period allows everything")

	env.Clock.Advance(time.Minute)
	owner = env.SeedSession(ownerID)
	blocked := rename(admin, org, "After the grace period")
	require.Equal(t, http.StatusForbidden, blocked.Status, string(blocked.Raw))
	assert.Equal(t, "SECOND_FACTOR_REQUIRED", blocked.Error.Code)
	invite := admin.Invite(org, "new@example.com", "viewer")
	assert.Equal(t, "SECOND_FACTOR_REQUIRED", errorCode(invite.Response), "member.manage is sensitive")
	assert.Equal(t, http.StatusOK, admin.Get(apptest.OrgPath(org, "")).Status, "reading stays allowed")
	assert.Equal(t, http.StatusOK, admin.Get(apptest.OrgPath(org, "/members")).Status, "reading stays allowed")
	assert.Equal(t, http.StatusOK, admin.Get(apptest.OrgPath(org, "/audit-events")).Status, "reading stays allowed")
	assert.Equal(t, http.StatusOK, rename(owner, org, "The owner has TOTP").Status)

	admin.EnableTotp(apptest.PasswordFactor("a long password for admin@example.com"))
	assert.Equal(t, http.StatusOK, rename(admin, org, "The admin set up TOTP").Status, "setting up a factor lifts the block at once")
	assert.Nil(t, admin.SecondFactorDue(org))
}

func TestWideningTheRuleStartsANewGracePeriodAndNarrowingKeepsIt(t *testing.T) {
	env := apptest.New(t)
	start, org, fresh := ownerWithTotp(t, env)
	ownerID := uuid.MustParse(start.UserID)
	_, adminID := env.SignedIn(org, "admin@example.com", "admin")
	_, developerID := env.SignedIn(org, "dev@example.com", "developer")
	owner := func() *apptest.Client { return env.SeedSession(ownerID) }
	dueOf := func(id uuid.UUID) *time.Time { return env.SeedSession(id).SecondFactorDue(org) }

	require.Equal(t, http.StatusOK, start.ChangeSecondFactorRule(org, "admins", fresh()).Status)
	first := *ruleOf(t, start, org).RequireSecondFactorSince
	env.Clock.Advance(3 * 24 * time.Hour)
	require.Equal(t, http.StatusOK, owner().ChangeSecondFactorRule(org, "everyone", fresh()).Status)
	widened := *ruleOf(t, owner(), org).RequireSecondFactorSince
	assert.True(t, widened.After(first), "the rule reached more roles, so the clock restarts")
	assert.WithinDuration(t, widened.Add(config.SecondFactorGrace), *dueOf(developerID), time.Second)
	assert.WithinDuration(t, widened.Add(config.SecondFactorGrace), *dueOf(adminID), time.Second,
		"one start for the whole rule keeps the model simple")

	env.Clock.Advance(time.Hour)
	require.Equal(t, http.StatusOK, owner().ChangeSecondFactorRule(org, "admins", fresh()).Status)
	assert.Equal(t, widened, *ruleOf(t, owner(), org).RequireSecondFactorSince, "narrowing never extends the grace period")
	assert.Nil(t, dueOf(developerID))

	env.Clock.Advance(time.Hour)
	require.Equal(t, http.StatusOK, owner().ChangeSecondFactorRule(org, "off", fresh()).Status)
	assert.Nil(t, ruleOf(t, owner(), org).RequireSecondFactorSince)
	assert.Nil(t, dueOf(adminID))

	var severity string
	require.NoError(t, env.Pool.QueryRow(t.Context(),
		`SELECT coalesce("Data"->>'Severity', '') FROM "AuditEvents" WHERE "Action" = 'identity.organization.second_factor_changed'
		 AND "Data"->>'To' = 'off'`).Scan(&severity))
	assert.Equal(t, "high", severity, "turning the rule off is a high-severity event")
}

func TestAMemberWhoJoinsLaterGetsTheirOwnGracePeriod(t *testing.T) {
	env := apptest.New(t)
	owner, org, fresh := ownerWithTotp(t, env)
	require.Equal(t, http.StatusOK, owner.ChangeSecondFactorRule(org, "everyone", fresh()).Status)
	env.Clock.Advance(30 * 24 * time.Hour)
	owner = env.SeedSession(uuid.MustParse(owner.UserID))

	issued := owner.MustInvite(org, "late@example.com", "admin")
	require.Equal(t, http.StatusOK, env.AcceptNew(issued.Token, "Late Joiner", "a long password for the late joiner").Status)
	late, res := env.Login("late@example.com", "a long password for the late joiner")
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	due := late.SecondFactorDue(org)
	require.NotNil(t, due)
	assert.WithinDuration(t, env.Clock.Now().Add(config.SecondFactorGrace), *due, 5*time.Second)
	assert.Equal(t, http.StatusOK, rename(late, org, "A new member in their grace period").Status)
}

func TestAnAPITokenOfAMemberWithoutAFactorIsBlockedAfterTheGracePeriod(t *testing.T) {
	env := apptest.New(t)
	owner, org, fresh := ownerWithTotp(t, env)
	_, adminID := env.SignedIn(org, "admin@example.com", "admin")
	token := env.SeedAPIToken(adminID, org, "org.read", "org.update", "member.manage")
	require.Equal(t, http.StatusOK, owner.ChangeSecondFactorRule(org, "admins", fresh()).Status)
	patch := func() apptest.Response {
		return env.Bearer(token.Value, apptest.Request{Method: http.MethodPatch, Path: apptest.OrgPath(org, ""), Body: map[string]string{"Name": "By token"}})
	}

	assert.Equal(t, http.StatusOK, patch().Status, "inside the grace period")
	env.Clock.Advance(config.SecondFactorGrace + time.Minute)
	blocked := patch()
	assert.Equal(t, http.StatusForbidden, blocked.Status, string(blocked.Raw))
	assert.Equal(t, "SECOND_FACTOR_REQUIRED", errorCode(blocked))
	read := env.Bearer(token.Value, apptest.Request{Method: http.MethodGet, Path: apptest.OrgPath(org, "")})
	assert.Equal(t, http.StatusOK, read.Status, "a token may still read")

	env.SeedPasskey(adminID)
	assert.Equal(t, http.StatusOK, patch().Status, "the token follows its creator's account")
}

func TestAMemberOfTwoOrganizationsFollowsEachRule(t *testing.T) {
	env := apptest.New(t)
	owner, strict, fresh := ownerWithTotp(t, env)
	relaxed := env.SeedOrganization("Relaxed Org", "relaxed-org")
	_, bothID := env.SignedIn(strict, "both@example.com", "admin")
	_, err := env.Pool.Exec(t.Context(), `INSERT INTO "Memberships" ("UserId", "OrganizationId", "Role") VALUES ($1, $2, 'owner')`, bothID, relaxed)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, owner.ChangeSecondFactorRule(strict, "admins", fresh()).Status)
	env.Clock.Advance(config.SecondFactorGrace + time.Minute)
	both := env.SeedSession(bothID)

	assert.Equal(t, "SECOND_FACTOR_REQUIRED", errorCode(rename(both, strict, "Strict")), "the strict organization blocks")
	assert.Equal(t, http.StatusOK, rename(both, relaxed, "Relaxed").Status, "the relaxed organization does not")
	assert.NotNil(t, both.SecondFactorDue(strict))
	assert.Nil(t, both.SecondFactorDue(relaxed))
}
