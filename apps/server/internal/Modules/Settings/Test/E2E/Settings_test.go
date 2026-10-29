package e2e_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

func TestNothingSetShowsTheDocumentedDefaults(t *testing.T) {
	env := apptest.New(t)
	owner, _ := env.SetupAndLogin()
	v := readSettings(t, owner)
	tests := []struct {
		group, field string
		want         any
	}{
		{"Mail", "Host", ""},
		{"Mail", "Port", int64(587)},
		{"Mail", "Tls", "starttls"},
		{"Organizations", "Creation", "instance-owner"},
		{"RateLimits", "Public", int64(300)},
		{"RateLimits", "InvitesPerUser", int64(50)},
		{"RateLimits", "InvitesPerOrganization", int64(200)},
		{"Logs", "RetentionDays", int64(30)},
		{"Logs", "JobLogMaxBytes", int64(64 << 20)},
	}
	for _, tt := range tests {
		s := v.field(t, tt.group, tt.field)
		assert.Equal(t, "default", s.Source, "%s.%s", tt.group, tt.field)
		assert.False(t, s.Locked)
		switch want := tt.want.(type) {
		case string:
			assert.Equal(t, want, s.text(t), "%s.%s", tt.group, tt.field)
		case int64:
			assert.Equal(t, want, s.number(t), "%s.%s", tt.group, tt.field)
		}
	}
	password := v.field(t, "Mail", "Password")
	assert.False(t, password.Set)
	assert.Equal(t, "DHOW_SMTP_PASSWORD", password.Env)
	assert.Nil(t, password.Value, "a secret has no value field at all")
	assert.Zero(t, v.version("Mail"))
}

func TestOnlyTheInstanceOwnerReadsOrChangesTheSettings(t *testing.T) {
	env := apptest.New(t)
	owner, d, fresh := ownerWithTotp(t, env)
	org := env.OrganizationID(d.OrgSlug)
	other := env.SeedOrganization("Other", "other")
	orgOwner, _ := env.SignedIn(other, "other-owner@example.com", "owner")
	admin, _ := env.SignedIn(org, "admin@example.com", "admin")
	for name, c := range map[string]*apptest.Client{"owner of another organization": orgOwner, "admin": admin} {
		res := c.Get("/instance/settings")
		assert.Equal(t, http.StatusForbidden, res.Status, name)
		res = put(c, "organizations", 0, fresh(), map[string]any{"Creation": "any-user"})
		assert.Equal(t, http.StatusForbidden, res.Status, name)
		res = c.Do(apptest.Request{Method: http.MethodPost, Path: "/instance/settings/mail/test"})
		assert.Equal(t, http.StatusForbidden, res.Status, name)
	}
	token := env.SeedAPIToken(uuid.MustParse(owner.UserID), org, "org.update")
	viaToken := env.Bearer(token.Value, apptest.Request{Method: http.MethodGet, Path: "/instance/settings"})
	assert.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, viaToken.Status, string(viaToken.Raw))
	anonymous := env.Do(apptest.Request{Method: http.MethodGet, Path: "/instance/settings"})
	assert.Equal(t, http.StatusUnauthorized, anonymous.Status)
	assert.Equal(t, "instance-owner", readSettings(t, owner).field(t, "Organizations", "Creation").text(t))
}

func TestASaveNeedsAFreshSecondFactor(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	res := put(owner, "organizations", 0, apptest.PasswordFactor(d.Password), map[string]any{"Creation": "any-user"})
	assert.Equal(t, http.StatusForbidden, res.Status, string(res.Raw))
	assert.Equal(t, "SECOND_FACTOR_REQUIRED", errorCode(res))

	enrollment := owner.EnableTotp(apptest.PasswordFactor(d.Password))
	tests := []struct {
		name   string
		factor map[string]any
		code   string
	}{
		{"no factor", map[string]any{}, ""},
		{"the password once a second factor is on", apptest.PasswordFactor(d.Password), "INVALID_CURRENT_FACTOR"},
		{"a wrong code", apptest.CodeFactor("totp", "000000"), "INVALID_CURRENT_FACTOR"},
		{"the code already used to turn TOTP on", apptest.CodeFactor("totp", env.TotpCode(enrollment.Secret, 0)), "INVALID_CURRENT_FACTOR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := put(owner, "organizations", 0, tt.factor, map[string]any{"Creation": "any-user"})
			assert.GreaterOrEqual(t, res.Status, 400, string(res.Raw))
			if tt.code != "" {
				assert.Equal(t, tt.code, errorCode(res), string(res.Raw))
			}
		})
	}
	assert.Equal(t, "instance-owner", readSettings(t, owner).field(t, "Organizations", "Creation").text(t))
	assert.Zero(t, env.AuditCount("system.settings.changed"))
}

func TestASettingSetInTheEnvironmentIsLocked(t *testing.T) {
	env := apptest.NewWithOptions(t, apptest.Options{Profile: func(c *config.Config) {
		c.SetEnvSetting(config.EnvRateLimitPublic, "1000")
	}})
	owner, _, fresh := ownerWithTotp(t, env)
	v := readSettings(t, owner)
	public := v.field(t, "RateLimits", "Public")
	assert.True(t, public.Locked)
	assert.Equal(t, "env", public.Source)
	assert.Equal(t, "DHOW_RATE_LIMIT_PUBLIC", public.Env)
	assert.Equal(t, int64(1000), public.number(t))

	res := put(owner, "rate-limits", 0, fresh(), map[string]any{"Public": 5, "InvitesPerUser": 10})
	require.Equal(t, http.StatusConflict, res.Status, string(res.Raw))
	assert.Equal(t, "SETTING_LOCKED_BY_ENV", errorCode(res))
	require.Len(t, res.Error.Details, 1)
	assert.Equal(t, "Public", res.Error.Details[0].Field)
	assert.Equal(t, int64(50), readSettings(t, owner).field(t, "RateLimits", "InvitesPerUser").number(t), "nothing is saved")

	v = save(t, owner, "rate-limits", fresh(), map[string]any{"InvitesPerUser": 10})
	assert.Equal(t, int64(10), v.field(t, "RateLimits", "InvitesPerUser").number(t))
	assert.Equal(t, "database", v.field(t, "RateLimits", "InvitesPerUser").Source)
	assert.Equal(t, int64(1000), v.field(t, "RateLimits", "Public").number(t))
}

func TestAStaleVersionIsRefused(t *testing.T) {
	env := apptest.New(t)
	owner, _, fresh := ownerWithTotp(t, env)
	save(t, owner, "logs", fresh(), map[string]any{"RetentionDays": 7})
	res := put(owner, "logs", 0, fresh(), map[string]any{"RetentionDays": 9})
	require.Equal(t, http.StatusConflict, res.Status, string(res.Raw))
	assert.Equal(t, "SETTING_CHANGED", errorCode(res))
	v := readSettings(t, owner)
	assert.Equal(t, int64(7), v.field(t, "Logs", "RetentionDays").number(t))
	assert.Equal(t, int64(1), v.version("Logs"))
	res = put(owner, "logs", 1, fresh(), map[string]any{"RetentionDays": 9})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
}

func TestEverySaveIsAuditedWithTheGroupAndTheFieldNames(t *testing.T) {
	env := apptest.New(t)
	owner, _, fresh := ownerWithTotp(t, env)
	save(t, owner, "rate-limits", fresh(), map[string]any{"Public": 400, "InvitesPerOrganization": 300})
	save(t, owner, "rate-limits", fresh(), map[string]any{"Public": 400})
	assert.Equal(t, 1, env.AuditCount("system.settings.changed"), "a save that changes nothing writes nothing")
	var raw []byte
	var target string
	var org *uuid.UUID
	require.NoError(t, env.Pool.QueryRow(t.Context(),
		`SELECT "Data", "TargetId", "OrganizationId" FROM "AuditEvents" WHERE "Action" = 'system.settings.changed'`).Scan(&raw, &target, &org))
	var data map[string]string
	require.NoError(t, json.Unmarshal(raw, &data))
	assert.Equal(t, map[string]string{"Group": "rate-limits", "Fields": "InvitesPerOrganization,Public", "Version": "1"}, data)
	assert.Equal(t, "rate-limits", target)
	assert.Nil(t, org, "the event belongs to the instance chain")
}
