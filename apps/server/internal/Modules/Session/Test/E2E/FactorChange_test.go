package e2e_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
)

func disable(c *apptest.Client, factor map[string]any) apptest.Response {
	return c.Do(apptest.Request{Method: http.MethodPost, Path: "/me/totp/disable", Body: map[string]any{"CurrentFactor": factor}})
}

func regenerate(c *apptest.Client, factor map[string]any) apptest.Response {
	return c.Do(apptest.Request{Method: http.MethodPost, Path: "/me/recovery-codes", Body: map[string]any{"CurrentFactor": factor}})
}

func TestDisablingTotpNeedsACurrentSecondFactor(t *testing.T) {
	env, c, d, enrolled := withTotp(t)
	other, _ := env.CompleteMfa(env.StartMfa(d.Email, d.Password), map[string]string{"Method": "totp", "Code": env.TotpCode(enrolled.Secret, 0)}, nil)
	env.Clock.Advance(31 * time.Second)

	tests := []struct {
		name   string
		factor map[string]any
	}{
		{"the password is not enough once a second factor exists", apptest.PasswordFactor(d.Password)},
		{"a wrong code", apptest.CodeFactor("totp", "000000")},
		{"an unknown recovery code", apptest.CodeFactor("recovery_code", "AAAA-BBBB-CCCC-DDDD")},
		{"a passkey assertion without a challenge", map[string]any{"Method": "passkey"}},
		{"an empty method", map[string]any{"Method": ""}},
	}
	for _, tt := range tests {
		res := disable(c, tt.factor)
		assert.Equal(t, http.StatusForbidden, res.Status, tt.name+": "+string(res.Raw))
		require.NotNil(t, res.Error, tt.name)
		assert.Equal(t, "INVALID_CURRENT_FACTOR", res.Error.Code, tt.name)
	}
	var status struct{ TotpEnabled bool }
	c.Get("/me/mfa").Decode(t, &status)
	assert.True(t, status.TotpEnabled, "nothing changed")

	env.Clock.Advance(15 * time.Minute)
	res := disable(c, apptest.CodeFactor("totp", env.TotpCode(enrolled.Secret, 0)))
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	assert.Equal(t, http.StatusUnauthorized, other.Get("/me").Status, "other sessions are revoked")
	var after struct {
		Enabled                bool
		RecoveryCodesRemaining int
	}
	c.Get("/me/mfa").Decode(t, &after)
	assert.False(t, after.Enabled)
	assert.Zero(t, after.RecoveryCodesRemaining, "recovery codes go with the last second factor")
	assert.Equal(t, http.StatusOK, env.PasswordStep(d.Email, d.Password, nil).Status, "the password alone signs in again")
	assert.Equal(t, 1, env.AuditCount("auth.totp_disabled"))
	assert.Positive(t, env.AuditCount("auth.mfa_failed"))

	again := disable(c, apptest.PasswordFactor(d.Password))
	assert.Equal(t, http.StatusConflict, again.Status)
}

func TestChangingTotpNeedsTheOldFactorAndReplacesIt(t *testing.T) {
	env, c, _, enrolled := withTotp(t)
	fresh := c.StartTotp()
	res := c.ConfirmTotp(env.TotpCode(fresh, 0), apptest.CodeFactor("totp", env.TotpCode(fresh, 0)))
	assert.Equal(t, "INVALID_CURRENT_FACTOR", res.Error.Code, "the new app's code does not prove the old factor")

	env.Clock.Advance(31 * time.Second)
	res = c.ConfirmTotp(env.TotpCode(fresh, 0), apptest.CodeFactor("totp", env.TotpCode(enrolled.Secret, 0)))
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var change struct{ RecoveryCodes []string }
	res.Decode(t, &change)
	assert.Empty(t, change.RecoveryCodes, "changing a factor keeps the existing recovery codes")

	env.Clock.Advance(31 * time.Second)
	old := disable(c, apptest.CodeFactor("totp", env.TotpCode(enrolled.Secret, 0)))
	assert.Equal(t, "INVALID_CURRENT_FACTOR", old.Error.Code, "the replaced secret stops working")
}

func TestTotpEnrollmentBelongsToItsSessionAndExpires(t *testing.T) {
	env := apptest.New(t)
	c, d := env.SetupAndLogin()
	other, _ := env.Login(d.Email, d.Password)
	secret := c.StartTotp()
	res := other.ConfirmTotp(env.TotpCode(secret, 0), apptest.PasswordFactor(d.Password))
	assert.Equal(t, http.StatusConflict, res.Status)
	assert.Equal(t, "ENROLLMENT_EXPIRED", res.Error.Code, "another session cannot finish this enrollment")

	env.Clock.Advance(10*time.Minute + time.Second)
	res = c.ConfirmTotp(env.TotpCode(secret, 0), apptest.PasswordFactor(d.Password))
	assert.Equal(t, "ENROLLMENT_EXPIRED", res.Error.Code)
}

func TestRegeneratingRecoveryCodesNeedsAFactorAndRetiresTheOldOnes(t *testing.T) {
	env, c, d, enrolled := withTotp(t)
	res := regenerate(c, apptest.PasswordFactor(d.Password))
	assert.Equal(t, "INVALID_CURRENT_FACTOR", res.Error.Code)

	res = regenerate(c, apptest.CodeFactor("recovery_code", enrolled.RecoveryCodes[0]))
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var change struct{ RecoveryCodes []string }
	res.Decode(t, &change)
	require.Len(t, change.RecoveryCodes, 10)
	assert.NotEqual(t, enrolled.RecoveryCodes, change.RecoveryCodes)
	assert.Equal(t, 1, env.AuditCount("auth.recovery_codes_regenerated"))
	assert.Equal(t, 1, env.AuditCount("auth.recovery_code_used"), "using a code to confirm is audited too")

	_, old := env.CompleteMfa(env.StartMfa(d.Email, d.Password), map[string]string{"Method": "recovery_code", "Code": enrolled.RecoveryCodes[5]}, nil)
	assert.Equal(t, "INVALID_MFA_CODE", old.Error.Code, "the old codes stop working")
	_, fresh := env.CompleteMfa(env.StartMfa(d.Email, d.Password), map[string]string{"Method": "recovery_code", "Code": change.RecoveryCodes[5]}, nil)
	assert.Equal(t, http.StatusOK, fresh.Status, string(fresh.Raw))
}

func TestRecoveryCodesNeedASecondFactor(t *testing.T) {
	env := apptest.New(t)
	c, d := env.SetupAndLogin()
	res := regenerate(c, apptest.PasswordFactor(d.Password))
	assert.Equal(t, http.StatusConflict, res.Status, string(res.Raw))
}
