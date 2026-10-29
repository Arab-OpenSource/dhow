package e2e_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	oidcmock "github.com/Khaymah/dhow/apps/server/internal/Core/Oidc/Mock"
	dbtest "github.com/Khaymah/dhow/apps/server/internal/Database/Testing"
)

func TestNoProviderShowsAnEmptyDatabaseList(t *testing.T) {
	o := newOidcEnv(t, apptest.Options{})
	v := readOidc(t, o.owner)
	assert.Equal(t, "database", v.Source)
	assert.False(t, v.Locked)
	assert.Equal(t, config.EnvOIDCProviders, v.Env)
	assert.Equal(t, apptest.PublicOrigin+config.OidcCallbackPath, v.CallbackBase)
	assert.Equal(t, 10, v.MaxProviders)
	assert.Empty(t, v.Providers)
	assert.Empty(t, publicProviders(t, o.env))
}

func TestTheOwnerAddsChangesAndRemovesAProviderWithAFreshFactor(t *testing.T) {
	o := newOidcEnv(t, apptest.Options{})
	created := o.addCorp(t, map[string]any{"AllowedDomains": []string{"Corp.Example", "corp.example"}})
	assert.Equal(t, "oidc", created.Type)
	assert.Equal(t, o.idp.Issuer(), created.Issuer)
	assert.Equal(t, []string{"corp.example"}, created.AllowedDomains)
	assert.True(t, created.Enabled)
	assert.True(t, created.ClientSecret.Set)
	assert.True(t, created.ClientSecret.Saved)
	assert.Equal(t, "DHOW_OIDC_CORP_CLIENT_SECRET", created.ClientSecret.Env)
	assert.Equal(t, apptest.PublicOrigin+"/oidc/callback/corp", created.CallbackUrl)
	assert.Equal(t, []string{"corp"}, publicProviders(t, o.env), "the sign-in page lists it at once")

	steps := []struct {
		name   string
		fields map[string]any
		check  func(t *testing.T, p oidcProviderView)
	}{
		{"rename", map[string]any{"Name": "Corp login"}, func(t *testing.T, p oidcProviderView) { assert.Equal(t, "Corp login", p.Name) }},
		{"domains", map[string]any{"AllowedDomains": []string{}}, func(t *testing.T, p oidcProviderView) { assert.Empty(t, p.AllowedDomains) }},
		{"client id", map[string]any{"ClientId": "another-client"}, func(t *testing.T, p oidcProviderView) { assert.Equal(t, "another-client", p.ClientId) }},
		{"an id in the body is ignored", map[string]any{"Id": "other"}, func(t *testing.T, p oidcProviderView) { assert.Equal(t, "corp", p.Id) }},
	}
	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			before := readOidc(t, o.owner).find(t, "corp")
			res := updateOidc(o.owner, "corp", before.Version, o.fresh(), s.fields)
			after := decodeOidc(t, res).find(t, "corp")
			s.check(t, after)
			assert.Equal(t, "corp", after.Id)
		})
	}

	stale := updateOidc(o.owner, "corp", created.Version, o.fresh(), map[string]any{"Name": "Old"})
	assert.Equal(t, http.StatusConflict, stale.Status)
	assert.Equal(t, "SETTING_CHANGED", errorCode(stale))

	current := readOidc(t, o.owner).find(t, "corp")
	removed := decodeOidc(t, removeOidc(o.owner, "corp", current.Version, o.fresh()))
	assert.Empty(t, removed.Providers)
	assert.Empty(t, publicProviders(t, o.env))
	var deleted bool
	require.NoError(t, o.env.Pool.QueryRow(t.Context(), `SELECT "IsDeleted" FROM "OidcProviders" WHERE "ProviderId" = 'corp'`).Scan(&deleted))
	assert.True(t, deleted, "removal is a soft delete")

	missing := removeOidc(o.owner, "corp", current.Version, o.fresh())
	assert.Equal(t, http.StatusNotFound, missing.Status)
}

func TestProviderWritesNeedTheOwnerASecondFactorAndAValidInput(t *testing.T) {
	o := newOidcEnv(t, apptest.Options{})
	member := o.env.SeedSession(o.seedMember(t, "dev@example.com"))
	tests := []struct {
		name   string
		client *apptest.Client
		factor map[string]any
		body   map[string]any
		status int
		code   string
		field  string
	}{
		{"a member", member, apptest.PasswordFactor("a long password for dev"), corp(o.idp, nil), http.StatusForbidden, "FORBIDDEN", ""},
		{"a wrong factor", o.owner, apptest.CodeFactor("totp", "000000"), corp(o.idp, nil), http.StatusForbidden, "INVALID_CURRENT_FACTOR", ""},
		{"a bad id", o.owner, nil, corp(o.idp, map[string]any{"Id": "Corp-1"}), http.StatusBadRequest, "SETTING_INVALID", "Id"},
		{"plain http", o.owner, nil, corp(o.idp, map[string]any{"Issuer": "http://sso.example.com"}), http.StatusBadRequest, "SETTING_INVALID", "Issuer"},
		{"a query", o.owner, nil, corp(o.idp, map[string]any{"Issuer": "https://sso.example.com/?a=1"}), http.StatusBadRequest, "SETTING_INVALID", "Issuer"},
		{"credentials", o.owner, nil, corp(o.idp, map[string]any{"Issuer": "https://u:p@sso.example.com"}), http.StatusBadRequest, "SETTING_INVALID", "Issuer"},
		{"metadata", o.owner, nil, corp(o.idp, map[string]any{"Issuer": "https://169.254.169.254/"}), http.StatusBadRequest, "SETTING_INVALID", "Issuer"},
		{"a metadata name", o.owner, nil, corp(o.idp, map[string]any{"Issuer": "https://metadata.google.internal"}), http.StatusBadRequest, "SETTING_INVALID", "Issuer"},
		{"a bad domain", o.owner, nil, corp(o.idp, map[string]any{"AllowedDomains": []string{"*.corp.example"}}), http.StatusBadRequest, "SETTING_INVALID", "AllowedDomains"},
		{"a long name", o.owner, nil, corp(o.idp, map[string]any{"Name": "abcdefghijabcdefghijabcdefghijabcdefghijX"}), http.StatusBadRequest, "SETTING_INVALID", "Name"},
		{"no secret", o.owner, nil, corp(o.idp, map[string]any{"ClientSecret": ""}), http.StatusBadRequest, "SETTING_INVALID", "ClientSecret"},
		{"a sample secret", o.owner, nil, corp(o.idp, map[string]any{"ClientSecret": "change-me-now"}), http.StatusBadRequest, "SETTING_INVALID", "ClientSecret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factor := tt.factor
			if factor == nil {
				factor = o.fresh()
			}
			res := createOidc(tt.client, factor, tt.body)
			assert.Equal(t, tt.status, res.Status, string(res.Raw))
			assert.Equal(t, tt.code, errorCode(res))
			if tt.field != "" {
				assert.Equal(t, []string{tt.field}, detailFields(res))
			}
		})
	}
	assert.Empty(t, readOidc(t, o.owner).Providers)

	noFactor := apptest.New(t)
	plainOwner, d := noFactor.SetupAndLogin()
	res := createOidc(plainOwner, apptest.PasswordFactor(d.Password), corp(o.idp, nil))
	assert.Equal(t, http.StatusForbidden, res.Status)
	assert.Equal(t, "SECOND_FACTOR_REQUIRED", errorCode(res))
}

func TestProviderIdsAreUniqueAndTheListHasALimit(t *testing.T) {
	o := newOidcEnv(t, apptest.Options{})
	o.addCorp(t, nil)
	again := createOidc(o.owner, o.fresh(), corp(o.idp, nil))
	assert.Equal(t, http.StatusConflict, again.Status)
	assert.Equal(t, "OIDC_PROVIDER_EXISTS", errorCode(again))

	shared := createOidc(o.owner, o.fresh(), corp(o.idp, map[string]any{"Id": "copy", "Issuer": o.idp.Issuer() + "/"}))
	assert.Equal(t, http.StatusBadRequest, shared.Status)
	assert.Equal(t, []string{"Issuer"}, detailFields(shared), "two providers never share an issuer")

	for i := range 9 {
		res := createOidc(o.owner, o.fresh(), corp(o.idp, map[string]any{
			"Id": "p" + string(rune('a'+i)), "Issuer": "https://sso" + string(rune('a'+i)) + ".example.com",
		}))
		require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	}
	eleventh := createOidc(o.owner, o.fresh(), corp(o.idp, map[string]any{"Id": "pz", "Issuer": "https://ssoz.example.com"}))
	assert.Equal(t, http.StatusConflict, eleventh.Status)
	assert.Equal(t, "OIDC_PROVIDER_LIMIT", errorCode(eleventh))
}

func TestTypeAndIssuerStayOnceAccountsAreLinked(t *testing.T) {
	o := newOidcEnv(t, apptest.Options{})
	p := o.addCorp(t, nil)
	moved := decodeOidc(t, updateOidc(o.owner, "corp", p.Version, o.fresh(), map[string]any{"Issuer": o.idp.Issuer() + "/"})).find(t, "corp")
	assert.Equal(t, o.idp.Issuer()+"/", moved.Issuer, "without linked accounts the issuer may change")
	back := decodeOidc(t, updateOidc(o.owner, "corp", moved.Version, o.fresh(), map[string]any{"Issuer": o.idp.Issuer()})).find(t, "corp")

	o.linkIdentity(t, o.seedMember(t, "linked@example.com"), "linked-1")
	assert.Equal(t, int64(1), readOidc(t, o.owner).find(t, "corp").LinkedIdentities)
	tests := []struct {
		name   string
		fields map[string]any
		want   []string
	}{
		{"issuer", map[string]any{"Issuer": "https://other.example.com"}, []string{"Issuer"}},
		{"type", map[string]any{"Type": "github"}, []string{"Type"}},
		{"both", map[string]any{"Type": "github", "Issuer": "https://github.example.com"}, []string{"Type", "Issuer"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := updateOidc(o.owner, "corp", back.Version, o.fresh(), tt.fields)
			assert.Equal(t, http.StatusConflict, res.Status, string(res.Raw))
			assert.Equal(t, "OIDC_PROVIDER_LINKED", errorCode(res))
			assert.ElementsMatch(t, tt.want, detailFields(res))
		})
	}
	renamed := decodeOidc(t, updateOidc(o.owner, "corp", back.Version, o.fresh(), map[string]any{"Name": "Still fine"}))
	assert.Equal(t, "Still fine", renamed.find(t, "corp").Name)

	current := renamed.find(t, "corp")
	decodeOidc(t, removeOidc(o.owner, "corp", current.Version, o.fresh()))
	reused := createOidc(o.owner, o.fresh(), corp(o.idp, map[string]any{"Issuer": "https://elsewhere.example.com"}))
	assert.Equal(t, http.StatusConflict, reused.Status)
	assert.Equal(t, "OIDC_PROVIDER_LINKED", errorCode(reused), "an old id keeps the issuer of its linked accounts")
	restored := decodeOidc(t, createOidc(o.owner, o.fresh(), corp(o.idp, nil))).find(t, "corp")
	assert.Equal(t, int64(1), restored.LinkedIdentities, "adding the same provider again brings the links back")
}

func TestTheEnvironmentListLocksEveryProvider(t *testing.T) {
	pool := dbtest.NewPool(t)
	o := newOidcEnv(t, apptest.Options{Pool: pool})
	o.addCorp(t, nil)

	locked := apptest.NewWithOptions(t, apptest.Options{Pool: pool, Clock: o.env.Clock, OidcTransport: o.idp.Transport(), Profile: envProvider(o.idp.Issuer() + "/staff")})
	owner := locked.SeedSession(o.userID(t, o.d.Email))
	v := readOidc(t, owner)
	assert.Equal(t, "env", v.Source)
	assert.True(t, v.Locked)
	assert.Equal(t, int64(1), v.IgnoredStored, "the saved provider is unused, not lost")
	require.Len(t, v.Providers, 1)
	staff := v.Providers[0]
	assert.Equal(t, "staff", staff.Id)
	assert.True(t, staff.ClientSecret.Set)
	assert.True(t, staff.ClientSecret.Locked)
	assert.False(t, staff.ClientSecret.Saved)
	assert.Equal(t, []string{"staff"}, publicProviders(t, locked), "never a mix of both sources")

	for name, res := range map[string]apptest.Response{
		"create": createOidc(owner, apptest.PasswordFactor(o.d.Password), corp(o.idp, map[string]any{"Id": "extra"})),
		"update": updateOidc(owner, "staff", 0, apptest.PasswordFactor(o.d.Password), map[string]any{"Name": "x"}),
		"remove": removeOidc(owner, "staff", 0, apptest.PasswordFactor(o.d.Password)),
	} {
		assert.Equal(t, http.StatusConflict, res.Status, name)
		assert.Equal(t, "SETTING_LOCKED_BY_ENV", errorCode(res), name)
	}
}

func TestAClientSecretIsSealedWriteOnlyAndNeverLeaks(t *testing.T) {
	mail := &apptest.FakeMailer{}
	o := newOidcEnv(t, apptest.Options{Mailer: mail})
	heard := listen(t, o.env)
	responses := []string{string(createOidc(o.owner, o.fresh(), corp(o.idp, nil)).Raw)}

	var sealed []byte
	require.NoError(t, o.env.Pool.QueryRow(t.Context(), `SELECT "Secret" FROM "OidcProviders" WHERE "ProviderId" = 'corp'`).Scan(&sealed))
	require.NotEmpty(t, sealed)
	assert.NotContains(t, string(sealed), oidcmock.ClientSecret)

	p := readOidc(t, o.owner).find(t, "corp")
	kept := updateOidc(o.owner, "corp", p.Version, o.fresh(), map[string]any{"Name": "Kept"})
	responses = append(responses, string(kept.Raw))
	p = decodeOidc(t, kept).find(t, "corp")
	assert.True(t, p.ClientSecret.Set, "leaving the field out keeps the secret")

	replaced := updateOidc(o.owner, "corp", p.Version, o.fresh(), map[string]any{"ClientSecret": "Nw4kR8tYp2Lm6Qz9Vb3Xc7Hd1Fg5Js0A"})
	responses = append(responses, string(replaced.Raw))
	p = decodeOidc(t, replaced).find(t, "corp")
	assert.Equal(t, "Nw4kR8tYp2Lm6Qz9Vb3Xc7Hd1Fg5Js0A", o.env.Wired.Settings.Current().Oidc.Active()[0].ClientSecret)

	cleared := updateOidc(o.owner, "corp", p.Version, o.fresh(), map[string]any{"ClientSecret": ""})
	responses = append(responses, string(cleared.Raw), string(o.owner.Get(oidcPath).Raw))
	p = decodeOidc(t, cleared).find(t, "corp")
	assert.False(t, p.ClientSecret.Set)
	assert.False(t, p.Enabled, "clearing the secret turns the provider off")
	assert.Empty(t, publicProviders(t, o.env))

	for _, r := range responses {
		assert.NotContains(t, r, oidcmock.ClientSecret)
		assert.NotContains(t, r, "Nw4kR8tYp2Lm6Qz9Vb3Xc7Hd1Fg5Js0A")
	}
	logs := o.env.Logs.String()
	assert.NotContains(t, logs, oidcmock.ClientSecret)
	assert.NotContains(t, logs, "Nw4kR8tYp2Lm6Qz9Vb3Xc7Hd1Fg5Js0A")
	rows, err := o.env.Pool.Query(t.Context(), `SELECT "Action", "Data"::text FROM "AuditEvents" WHERE "Action" = 'system.settings.changed' ORDER BY "CreatedAt"`)
	require.NoError(t, err)
	type event struct{ Action, Data string }
	audit, err := pgx.CollectRows(rows, pgx.RowToStructByPos[event])
	require.NoError(t, err)
	require.Len(t, audit, 4)
	assert.Contains(t, audit[0].Data, `"Operation": "create"`)
	assert.Contains(t, audit[2].Data, `"Fields": "ClientSecret"`)
	for _, a := range audit {
		assert.Contains(t, a.Data, `"Group": "oidc"`)
		assert.Contains(t, a.Data, `"Provider": "corp"`)
		assert.NotContains(t, a.Data, oidcmock.ClientSecret)
		assert.NotContains(t, a.Data, "Nw4kR8tYp2Lm6Qz9Vb3Xc7Hd1Fg5Js0A")
	}
	require.Eventually(t, func() bool { return len(heard.all()) >= 4 }, 5*time.Second, 20*time.Millisecond)
	for _, n := range heard.all() {
		assert.Equal(t, "oidc", n, "a notification names the group only")
	}
	for _, m := range o.env.SentMail(mail) {
		assert.NotContains(t, m.Subject+m.Body, oidcmock.ClientSecret)
	}
}
