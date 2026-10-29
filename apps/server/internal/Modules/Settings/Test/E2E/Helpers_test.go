package e2e_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

const totpStep = 30 * time.Second

type setting struct {
	Value  json.RawMessage
	Source string
	Locked bool
	Env    string
	Set    bool
}

type settingsView map[string]map[string]json.RawMessage

func (v settingsView) version(group string) int64 {
	var n int64
	if raw, ok := v[group]["Version"]; ok {
		_ = json.Unmarshal(raw, &n)
	}
	return n
}

func (v settingsView) field(t *testing.T, group, field string) setting {
	t.Helper()
	raw, ok := v[group][field]
	require.True(t, ok, "%s.%s is missing", group, field)
	var s setting
	require.NoError(t, json.Unmarshal(raw, &s))
	return s
}

func (s setting) text(t *testing.T) string {
	t.Helper()
	var out string
	require.NoError(t, json.Unmarshal(s.Value, &out))
	return out
}

func (s setting) number(t *testing.T) int64 {
	t.Helper()
	var out int64
	require.NoError(t, json.Unmarshal(s.Value, &out))
	return out
}

func readSettings(t *testing.T, c *apptest.Client) settingsView {
	t.Helper()
	res := c.Get("/instance/settings")
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var v settingsView
	res.Decode(t, &v)
	return v
}

var groupKeys = map[string]string{
	"mail": "Mail", "organizations": "Organizations", "passwords": "Passwords", "rate-limits": "RateLimits", "logs": "Logs",
}

func put(c *apptest.Client, group string, version int64, factor map[string]any, fields map[string]any) apptest.Response {
	body := map[string]any{"Version": version, "CurrentFactor": factor}
	for k, v := range fields {
		body[k] = v
	}
	return c.Do(apptest.Request{Method: http.MethodPut, Path: "/instance/settings/" + group, Body: body})
}

func save(t *testing.T, c *apptest.Client, group string, factor map[string]any, fields map[string]any) settingsView {
	t.Helper()
	version := readSettings(t, c).version(groupKeys[group])
	res := put(c, group, version, factor, fields)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var v settingsView
	res.Decode(t, &v)
	return v
}

func errorCode(res apptest.Response) string {
	if res.Error == nil {
		return ""
	}
	return res.Error.Code
}

func ownerWithTotp(t *testing.T, env *apptest.Env) (*apptest.Client, apptest.SetupDetails, func() map[string]any) {
	t.Helper()
	owner, d := env.SetupAndLogin()
	enrollment := owner.EnableTotp(apptest.PasswordFactor(d.Password))
	fresh := func() map[string]any {
		env.Clock.Advance(totpStep)
		return apptest.CodeFactor("totp", env.TotpCode(enrollment.Secret, 0))
	}
	return owner, d, fresh
}
