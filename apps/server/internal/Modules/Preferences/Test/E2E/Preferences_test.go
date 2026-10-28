package e2e_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	preferences "github.com/Khaymah/dhow/apps/server/internal/Modules/Preferences"
)

type prefs struct {
	Locale         string
	ThemeId        string
	CustomThemes   []map[string]any
	OrganizationId *string
	Saved          bool
	UpdatedAt      *string
}

func colors(hex string) map[string]string {
	out := map[string]string{}
	for _, k := range preferences.ThemeTokens() {
		out[k] = hex
	}
	return out
}

func body(locale, theme string, org any, themes ...map[string]any) map[string]any {
	if themes == nil {
		themes = []map[string]any{}
	}
	return map[string]any{"Locale": locale, "ThemeId": theme, "CustomThemes": themes, "OrganizationId": org}
}

func save(c *apptest.Client, b map[string]any) apptest.Response {
	return c.Do(apptest.Request{Method: http.MethodPut, Path: "/me/preferences", Body: b})
}

func TestPreferencesDefaultThenSaved(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug).String()

	res := owner.Get("/me/preferences")
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var got prefs
	res.Decode(t, &got)
	assert.Equal(t, prefs{Locale: "ar", ThemeId: "sea", CustomThemes: []map[string]any{}}, got)

	mine := map[string]any{"Id": "c1abc", "Name": "ليلي", "Colors": colors("#123456")}
	saved := save(owner, body("en", "c1abc", org, mine))
	require.Equal(t, http.StatusOK, saved.Status, string(saved.Raw))
	assert.Equal(t, "preferences.saved", saved.Message)
	saved.Decode(t, &got)
	assert.True(t, got.Saved)
	assert.Equal(t, "en", got.Locale)
	assert.Equal(t, "c1abc", got.ThemeId)
	require.NotNil(t, got.OrganizationId)
	assert.Equal(t, org, *got.OrganizationId)
	require.Len(t, got.CustomThemes, 1)
	assert.Equal(t, "ليلي", got.CustomThemes[0]["Name"])
	var me struct{ User struct{ Locale string } }
	owner.Get("/me").Decode(t, &me)
	assert.Equal(t, "en", me.User.Locale, "the language is the account locale")

	other, _ := env.Login(d.Email, d.Password)
	var again prefs
	other.Get("/me/preferences").Decode(t, &again)
	assert.Equal(t, got.ThemeId, again.ThemeId, "another device sees the same preferences")
}

func TestPreferencesValidation(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	otherOrg := env.SeedOrganization("Other", "other").String()
	bad := colors("#123456")
	bad["a1"] = "#12345G"
	missing := colors("#123456")
	delete(missing, "violet-soft")
	extra := colors("#123456")
	extra["shadow"] = "#000000"
	tests := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"unknown theme", body("ar", "rainbow", nil), "ThemeId"},
		{"unknown locale", body("fr", "sea", nil), "Locale"},
		{"a custom theme with a system id", body("ar", "sea", nil, map[string]any{"Id": "sea", "Name": "x", "Colors": colors("#123456")}), "CustomThemes[0]"},
		{"bad color", body("ar", "sea", nil, map[string]any{"Id": "c1", "Name": "x", "Colors": bad}), "CustomThemes[0]"},
		{"upper-case color", body("ar", "sea", nil, map[string]any{"Id": "c1", "Name": "x", "Colors": colors("#ABCDEF")}), "CustomThemes[0]"},
		{"missing color", body("ar", "sea", nil, map[string]any{"Id": "c1", "Name": "x", "Colors": missing}), "CustomThemes[0]"},
		{"unknown color", body("ar", "sea", nil, map[string]any{"Id": "c1", "Name": "x", "Colors": extra}), "CustomThemes[0]"},
		{"empty name", body("ar", "sea", nil, map[string]any{"Id": "c1", "Name": "", "Colors": colors("#123456")}), "CustomThemes[0]"},
		{"long name", body("ar", "sea", nil, map[string]any{"Id": "c1", "Name": strings.Repeat("ن", 41), "Colors": colors("#123456")}), "CustomThemes[0]"},
		{"duplicate ids", body("ar", "sea", nil,
			map[string]any{"Id": "c1", "Name": "x", "Colors": colors("#123456")},
			map[string]any{"Id": "c1", "Name": "y", "Colors": colors("#123456")}), "CustomThemes[1]"},
		{"an organization the user is not in", body("ar", "sea", otherOrg), "OrganizationId"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := save(owner, tt.body)
			assert.Equal(t, http.StatusUnprocessableEntity, res.Status, string(res.Raw))
			require.NotNil(t, res.Error)
			require.NotEmpty(t, res.Error.Details)
			assert.Equal(t, tt.field, res.Error.Details[0].Field)
		})
	}
	many := make([]map[string]any, 51)
	for i := range many {
		many[i] = map[string]any{"Id": "c" + strconv.Itoa(i), "Name": "x", "Colors": colors("#123456")}
	}
	tooMany := save(owner, body("ar", "sea", nil, many...))
	assert.Equal(t, http.StatusUnprocessableEntity, tooMany.Status)
	assert.Equal(t, "CustomThemes", tooMany.Error.Details[0].Field)
	assert.Equal(t, http.StatusOK, save(owner, body("ar", "c49", nil, many[:50]...)).Status, "fifty themes fit in the body limit")
	_, err := env.Pool.Exec(context.Background(), `UPDATE "UserPreferences" SET "IsDeleted" = true`)
	require.NoError(t, err)
	var n int
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT count(*) FROM "UserPreferences" WHERE NOT "IsDeleted"`).Scan(&n))
	assert.Zero(t, n, "nothing invalid is stored")
	_, login := env.Login(d.Email, d.Password)
	assert.Equal(t, http.StatusOK, login.Status)
}

func TestPreferencesForgetAnOrganizationTheUserLeft(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	member, id := env.SignedIn(org, "m@example.com", "viewer")
	require.Equal(t, http.StatusOK, save(member, body("ar", "dusk", org.String())).Status)
	res := owner.Do(apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(org, "/members/"+id.String())})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	member, _ = env.Login("m@example.com", "a long password for m@example.com")
	var got prefs
	member.Get("/me/preferences").Decode(t, &got)
	assert.Nil(t, got.OrganizationId)
	assert.Equal(t, "dusk", got.ThemeId)
}

func TestPreferencesNeedASessionAndCSRF(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	anonymous := env.Do(apptest.Request{Method: http.MethodGet, Path: "/me/preferences"})
	assert.Equal(t, http.StatusUnauthorized, anonymous.Status)
	noToken := owner.Do(apptest.Request{Method: http.MethodPut, Path: "/me/preferences", Body: body("ar", "sea", nil), Headers: map[string]string{"X-CSRF-Token": ""}})
	assert.Equal(t, http.StatusForbidden, noToken.Status)
	token := owner.MustCreateAPIToken(env.OrganizationID(d.OrgSlug), []string{"org.read"}, d.Password)
	viaToken := env.Bearer(token.Value, apptest.Request{Method: http.MethodPut, Path: "/me/preferences", Body: body("en", "sea", nil)})
	assert.Equal(t, http.StatusForbidden, viaToken.Status)
	assert.Equal(t, "SESSION_REQUIRED", viaToken.Error.Code)
}
