package settings

import (
	"context"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	oidcauth "github.com/Khaymah/dhow/apps/server/internal/Core/Oidc"
	instancesettings "github.com/Khaymah/dhow/apps/server/internal/Core/Settings"
	gen "github.com/Khaymah/dhow/apps/server/internal/Modules/Settings/Gen"
)

const (
	messageOidcLoaded  = "settings.oidc.loaded"
	messageOidcSaved   = "settings.oidc.saved"
	messageOidcRemoved = "settings.oidc.removed"
	messageOidcTested  = "settings.oidc.tested"
	envSecretSuffix    = "_CLIENT_SECRET"
	oidcMaxProviders   = settingspec.OIDCMaxProviders
)

func oidcToGen(v OidcView) gen.OidcSettings {
	o := v.Snapshot.Oidc
	out := gen.OidcSettings{
		Source: gen.SettingSource(o.Source), Locked: o.Locked(), Env: config.EnvOIDCProviders, CallbackBase: v.CallbackBase,
		MaxProviders: oidcMaxProviders, IgnoredStored: v.IgnoredStored, Providers: make([]gen.OidcProviderSetting, 0, len(o.Providers)),
	}
	for _, p := range o.Providers {
		c := p.Config
		domains := append([]string{}, c.AllowedDomains...)
		out.Providers = append(out.Providers, gen.OidcProviderSetting{
			Id: c.ID, Type: gen.OidcProviderType(c.Kind), Name: c.Name, Issuer: c.Issuer, ClientId: c.ClientID,
			ClientSecret: gen.SecretSetting{
				Set: p.SecretSet, Saved: !o.Locked() && p.SecretSet, Locked: o.Locked(), Env: config.OIDCEnvName(c.ID, envSecretSuffix),
			},
			AllowedDomains: domains, Enabled: p.Enabled(), Version: p.Version, LinkedIdentities: v.Linked[c.ID],
			OnlySignInUsers: v.OnlySignIn[c.ID], CallbackUrl: v.CallbackBase + c.ID, UpdatedAt: p.UpdatedAt,
		})
	}
	return out
}

func text(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func domainsOf(v *[]string) []string {
	if v == nil {
		return nil
	}
	return *v
}

func kindOf(v *gen.OidcProviderType) *string {
	if v == nil {
		return nil
	}
	s := string(*v)
	return &s
}

func (c *Controller) GetOidcSettings(ctx context.Context, _ gen.GetOidcSettingsRequestObject) (gen.GetOidcSettingsResponseObject, error) {
	view, err := c.service.GetOidc(ctx)
	if err != nil {
		return nil, err
	}
	data := oidcToGen(view)
	return gen.GetOidcSettings200JSONResponse{Success: true, Message: ptr(messageOidcLoaded), Data: &data}, nil
}

func (c *Controller) CreateOidcProvider(ctx context.Context, req gen.CreateOidcProviderRequestObject) (gen.CreateOidcProviderResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	b := req.Body
	view, err := c.service.ChangeOidc(ctx, instancesettings.OidcChange{Operation: instancesettings.OidcCreate, Input: instancesettings.OidcInput{
		ID: b.Id, Kind: string(b.Type), Name: text(b.Name), Issuer: text(b.Issuer), ClientID: b.ClientId, ClientSecret: text(b.ClientSecret),
		AllowedDomains: domainsOf(b.AllowedDomains),
	}}, factorOf(b.CurrentFactor))
	if err != nil {
		return nil, err
	}
	data := oidcToGen(view)
	return gen.CreateOidcProvider200JSONResponse{Success: true, Message: ptr(messageOidcSaved), Data: &data}, nil
}

func (c *Controller) UpdateOidcProvider(ctx context.Context, req gen.UpdateOidcProviderRequestObject) (gen.UpdateOidcProviderResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	b := req.Body
	view, err := c.service.ChangeOidc(ctx, instancesettings.OidcChange{
		Operation: instancesettings.OidcUpdate, ID: req.ProviderId, Version: b.Version,
		Patch: instancesettings.OidcPatch{
			Kind: kindOf(b.Type), Name: b.Name, Issuer: b.Issuer, ClientID: b.ClientId, ClientSecret: b.ClientSecret, AllowedDomains: b.AllowedDomains,
		},
	}, factorOf(b.CurrentFactor))
	if err != nil {
		return nil, err
	}
	data := oidcToGen(view)
	return gen.UpdateOidcProvider200JSONResponse{Success: true, Message: ptr(messageOidcSaved), Data: &data}, nil
}

func (c *Controller) RemoveOidcProvider(ctx context.Context, req gen.RemoveOidcProviderRequestObject) (gen.RemoveOidcProviderResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	view, err := c.service.ChangeOidc(ctx, instancesettings.OidcChange{
		Operation: instancesettings.OidcRemove, ID: req.ProviderId, Version: req.Body.Version,
	}, factorOf(req.Body.CurrentFactor))
	if err != nil {
		return nil, err
	}
	data := oidcToGen(view)
	return gen.RemoveOidcProvider200JSONResponse{Success: true, Message: ptr(messageOidcRemoved), Data: &data}, nil
}

func (c *Controller) TestOidcProvider(ctx context.Context, req gen.TestOidcProviderRequestObject) (gen.TestOidcProviderResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	res, err := c.service.TestOidc(ctx, string(req.Body.Type), text(req.Body.Issuer))
	if err != nil {
		return nil, err
	}
	data := testToGen(res)
	return gen.TestOidcProvider200JSONResponse{Success: true, Message: ptr(messageOidcTested), Data: &data}, nil
}

func testToGen(res oidcauth.ProbeResult) gen.OidcTest {
	out := gen.OidcTest{Passed: res.Passed(), Checks: make([]gen.OidcTestCheck, 0, len(res.Checks))}
	for _, c := range res.Checks {
		out.Checks = append(out.Checks, gen.OidcTestCheck{Check: gen.OidcTestCheckCheck(c.Check), Passed: c.Passed})
	}
	return out
}
