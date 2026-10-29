package e2e_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	oidcmock "github.com/Khaymah/dhow/apps/server/internal/Core/Oidc/Mock"
)

const oidcPath = "/instance/settings/oidc"

type oidcProviderView struct {
	Id, Type, Name, Issuer, ClientId string
	ClientSecret                     struct {
		Set, Saved, Locked bool
		Env                string
	}
	AllowedDomains                    []string
	Enabled                           bool
	Version                           int64
	LinkedIdentities, OnlySignInUsers int64
	CallbackUrl                       string
}

type oidcView struct {
	Source        string
	Locked        bool
	Env           string
	CallbackBase  string
	MaxProviders  int
	IgnoredStored int64
	Providers     []oidcProviderView
}

func (v oidcView) find(t *testing.T, id string) oidcProviderView {
	t.Helper()
	for _, p := range v.Providers {
		if p.Id == id {
			return p
		}
	}
	require.Failf(t, "provider missing", "%s is not in %+v", id, v.Providers)
	return oidcProviderView{}
}

func decodeOidc(t *testing.T, res apptest.Response) oidcView {
	t.Helper()
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var v oidcView
	res.Decode(t, &v)
	return v
}

func readOidc(t *testing.T, c *apptest.Client) oidcView {
	t.Helper()
	return decodeOidc(t, c.Get(oidcPath))
}

type oidcEnv struct {
	env   *apptest.Env
	idp   *oidcmock.IdP
	owner *apptest.Client
	d     apptest.SetupDetails
	fresh func() map[string]any
}

func newOidcEnv(t *testing.T, o apptest.Options) oidcEnv {
	t.Helper()
	clock := o.Clock
	if clock == nil {
		clock = apptest.NewClock(time.Now().UTC().Truncate(time.Second))
		o.Clock = clock
	}
	idp := oidcmock.New(t, false, clock.Now)
	o.OidcTransport = idp.Transport()
	env := apptest.NewWithOptions(t, o)
	owner, d, fresh := ownerWithTotp(t, env)
	return oidcEnv{env: env, idp: idp, owner: owner, d: d, fresh: fresh}
}

func corp(idp *oidcmock.IdP, extra map[string]any) map[string]any {
	body := map[string]any{
		"Id": apptest.OidcProvider, "Type": "oidc", "Name": "Corp SSO", "Issuer": idp.Issuer(), "ClientId": oidcmock.ClientID,
		"ClientSecret": oidcmock.ClientSecret,
	}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

func createOidc(c *apptest.Client, factor map[string]any, body map[string]any) apptest.Response {
	body["CurrentFactor"] = factor
	return c.Do(apptest.Request{Method: http.MethodPost, Path: oidcPath, Body: body})
}

func updateOidc(c *apptest.Client, id string, version int64, factor map[string]any, fields map[string]any) apptest.Response {
	body := map[string]any{"Version": version, "CurrentFactor": factor}
	for k, v := range fields {
		body[k] = v
	}
	return c.Do(apptest.Request{Method: http.MethodPut, Path: oidcPath + "/" + id, Body: body})
}

func removeOidc(c *apptest.Client, id string, version int64, factor map[string]any) apptest.Response {
	return c.Do(apptest.Request{Method: http.MethodPost, Path: oidcPath + "/" + id + "/remove", Body: map[string]any{
		"Version": version, "CurrentFactor": factor,
	}})
}

func (o oidcEnv) addCorp(t *testing.T, extra map[string]any) oidcProviderView {
	t.Helper()
	return decodeOidc(t, createOidc(o.owner, o.fresh(), corp(o.idp, extra))).find(t, apptest.OidcProvider)
}

func (o oidcEnv) linkIdentity(t *testing.T, user uuid.UUID, subject string) {
	t.Helper()
	_, err := o.env.Pool.Exec(t.Context(),
		`INSERT INTO "UserIdentities" ("UserId", "Provider", "Issuer", "Subject", "Email") VALUES ($1, $2, $3, $4, $5)`,
		user, apptest.OidcProvider, o.idp.Issuer(), subject, subject+"@corp.example")
	require.NoError(t, err)
}

func (o oidcEnv) userID(t *testing.T, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, o.env.Pool.QueryRow(t.Context(), `SELECT "Id" FROM "Users" WHERE lower("Email") = lower($1)`, email).Scan(&id))
	return id
}

func (o oidcEnv) dropPassword(t *testing.T, user uuid.UUID) {
	t.Helper()
	_, err := o.env.Pool.Exec(t.Context(), `UPDATE "Users" SET "PasswordHash" = '' WHERE "Id" = $1`, user)
	require.NoError(t, err)
}

func (o oidcEnv) seedMember(t *testing.T, email string) uuid.UUID {
	t.Helper()
	return o.env.SeedUser(o.env.OrganizationID(o.d.OrgSlug), email, "a long password for "+strings.Split(email, "@")[0], "developer")
}

func publicProviders(t *testing.T, env *apptest.Env) []string {
	t.Helper()
	res := env.Do(apptest.Request{Method: http.MethodGet, Path: "/oidc/providers"})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var list []struct{ Id string }
	res.Decode(t, &list)
	out := []string{}
	for _, p := range list {
		out = append(out, p.Id)
	}
	return out
}

func envProvider(issuer string) func(*config.Config) {
	return func(c *config.Config) {
		c.OIDCProviders = []config.OIDCProvider{{
			ID: "staff", Kind: config.OIDCKindOIDC, Name: "Staff SSO", Issuer: issuer, ClientID: oidcmock.ClientID,
			ClientSecret: oidcmock.ClientSecret,
		}}
	}
}
