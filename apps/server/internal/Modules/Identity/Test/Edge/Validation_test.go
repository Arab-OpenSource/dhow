package edge_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

func TestSetupValidation(t *testing.T) {
	env := apptest.New(t)
	token := env.SetupToken()
	tests := []struct {
		name  string
		edit  func(*apptest.SetupDetails)
		field string
	}{
		{"empty owner name", func(d *apptest.SetupDetails) { d.OwnerName = "" }, "OwnerName"},
		{"owner name with a control character", func(d *apptest.SetupDetails) { d.OwnerName = "Mona\x00" }, "OwnerName"},
		{"owner name too long", func(d *apptest.SetupDetails) { d.OwnerName = strings.Repeat("م", 101) }, "OwnerName"},
		{"email without an at sign", func(d *apptest.SetupDetails) { d.Email = "owner.example.com" }, "Email"},
		{"email with a display name", func(d *apptest.SetupDetails) { d.Email = "Owner <o@example.com>" }, "Email"},
		{"empty email", func(d *apptest.SetupDetails) { d.Email = "" }, "Email"},
		{"empty organization name", func(d *apptest.SetupDetails) { d.OrgName = "" }, "OrganizationName"},
		{"slug with capitals is not normalized silently", func(d *apptest.SetupDetails) { d.OrgSlug = "Dhow" }, "OrganizationSlug"},
		{"slug with spaces", func(d *apptest.SetupDetails) { d.OrgSlug = "dhow co" }, "OrganizationSlug"},
		{"slug with underscores", func(d *apptest.SetupDetails) { d.OrgSlug = "dhow_co" }, "OrganizationSlug"},
		{"slug with a double hyphen", func(d *apptest.SetupDetails) { d.OrgSlug = "dhow--co" }, "OrganizationSlug"},
		{"slug starting with a hyphen", func(d *apptest.SetupDetails) { d.OrgSlug = "-dhow" }, "OrganizationSlug"},
		{"slug too long", func(d *apptest.SetupDetails) { d.OrgSlug = strings.Repeat("a", 64) }, "OrganizationSlug"},
		{"empty slug", func(d *apptest.SetupDetails) { d.OrgSlug = "" }, "OrganizationSlug"},
		{"password too short", func(d *apptest.SetupDetails) { d.Password = "short" }, "Password"},
		{"password too long", func(d *apptest.SetupDetails) { d.Password = strings.Repeat("a", 129) }, "Password"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := apptest.DefaultSetup()
			d.Token = token
			tt.edit(&d)
			res := env.Setup(d)
			assert.Equal(t, http.StatusUnprocessableEntity, res.Status, string(res.Raw))
			require.NotNil(t, res.Error)
			assert.Equal(t, "VALIDATION_FAILED", res.Error.Code)
			fields := make([]string, 0)
			for _, detail := range res.Error.Details {
				fields = append(fields, detail.Field)
			}
			assert.Contains(t, fields, tt.field)
		})
	}
}

func TestSetupNeverAcceptsARole(t *testing.T) {
	env := apptest.New(t)
	d := apptest.DefaultSetup()
	d.Token = env.SetupToken()
	res := env.Do(apptest.Request{Method: http.MethodPost, Path: "/setup", Headers: map[string]string{"Origin": apptest.PublicOrigin}, Body: map[string]any{
		"SetupToken": d.Token, "OwnerName": d.OwnerName, "Email": d.Email, "Password": d.Password,
		"OrganizationName": d.OrgName, "OrganizationSlug": d.OrgSlug, "Role": "viewer", "IsAdmin": true, "Disabled": true,
	}})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var role string
	var disabled bool
	require.NoError(t, env.Pool.QueryRow(t.Context(), `SELECT m."Role", u."Disabled" FROM "Memberships" m JOIN "Users" u ON u."Id" = m."UserId"`).Scan(&role, &disabled))
	assert.Equal(t, "owner", role, "the DTO has no role field, so a client cannot choose one")
	assert.False(t, disabled)
}

func TestSetupLocale(t *testing.T) {
	tests := []struct {
		name   string
		header string
		body   string
		want   string
	}{
		{"defaults to Arabic", "", "", "ar"},
		{"follows Accept-Language", "en-US,en;q=0.9", "", "en"},
		{"an explicit choice wins", "en", "ar", "ar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := apptest.New(t)
			d := apptest.DefaultSetup()
			body := map[string]any{
				"SetupToken": env.SetupToken(), "OwnerName": d.OwnerName, "Email": d.Email, "Password": d.Password,
				"OrganizationName": d.OrgName, "OrganizationSlug": d.OrgSlug,
			}
			if tt.body != "" {
				body["Locale"] = tt.body
			}
			res := env.Do(apptest.Request{Method: http.MethodPost, Path: "/setup", Headers: map[string]string{"Origin": apptest.PublicOrigin, "Accept-Language": tt.header}, Body: body})
			require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
			var out struct{ User struct{ Locale string } }
			res.Decode(t, &out)
			assert.Equal(t, tt.want, out.User.Locale)
		})
	}
	t.Run("an unsupported locale is refused", func(t *testing.T) {
		env := apptest.New(t)
		d := apptest.DefaultSetup()
		res := env.Do(apptest.Request{Method: http.MethodPost, Path: "/setup", Headers: map[string]string{"Origin": apptest.PublicOrigin}, Body: map[string]any{
			"SetupToken": env.SetupToken(), "OwnerName": d.OwnerName, "Email": d.Email, "Password": d.Password,
			"OrganizationName": d.OrgName, "OrganizationSlug": d.OrgSlug, "Locale": "fr",
		}})
		assert.Equal(t, http.StatusUnprocessableEntity, res.Status)
	})
}

func TestErrorMessagesFollowAcceptLanguage(t *testing.T) {
	env := apptest.New(t)
	for lang, want := range map[string]string{
		"en": "The email or password is not correct.",
		"ar": "البريد الإلكتروني أو كلمة السر غير صحيحة.",
	} {
		res := env.Do(apptest.Request{
			Method: http.MethodPost, Path: "/sessions", Headers: map[string]string{"Origin": apptest.PublicOrigin, "Accept-Language": lang},
			Body: map[string]string{"Email": "x@example.com", "Password": "whatever whatever"},
		})
		assert.Equal(t, want, res.Error.Message, lang)
	}
}
