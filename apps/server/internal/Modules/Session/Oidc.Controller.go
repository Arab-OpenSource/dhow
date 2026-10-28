package session

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	gen "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session/Gen"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session/Models"
)

const (
	messageOidcProviders  = "oidc.providers.listed"
	messageOidcAuthorize  = "oidc.authorize.ready"
	messageIdentities     = "oidc.identities.listed"
	messageIdentityLinked = "oidc.identity.linked"
	messageIdentityGone   = "oidc.identity.removed"
)

func oidcCookie(value string) string {
	return auth.SessionCookie(config.OidcCookieName, value, config.OidcFlowTTL)
}

func authorizationToGen(a models.OidcAuthorization) gen.OidcAuthorizationResponse {
	return gen.OidcAuthorizationResponse{
		Success: true, Message: ptr(messageOidcAuthorize),
		Data: &gen.OidcAuthorization{AuthorizationUrl: a.URL, ExpiresAt: a.ExpiresAt},
	}
}

func identityToGen(i models.LinkedIdentity) gen.LinkedIdentity {
	return gen.LinkedIdentity{
		Id: i.ID, Provider: i.Provider, ProviderName: i.ProviderName, Email: i.Email, CreatedAt: i.CreatedAt, LastUsedAt: i.LastUsedAt,
	}
}

func callbackOf(b *gen.OidcCallbackRequest) models.OidcCallback {
	return models.OidcCallback{Provider: b.Provider, Code: b.Code, State: b.State}
}

type cookieResponse struct {
	status  int
	body    any
	cookies []string
}

func (r cookieResponse) write(w http.ResponseWriter) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r.body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	for _, c := range r.cookies {
		w.Header().Add("Set-Cookie", c)
	}
	w.WriteHeader(r.status)
	_, err := buf.WriteTo(w)
	return err
}

func (r cookieResponse) VisitCompleteOidcLoginResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (c *Controller) ListOidcProviders(_ context.Context, _ gen.ListOidcProvidersRequestObject) (gen.ListOidcProvidersResponseObject, error) {
	list := c.service.ListOidcProviders()
	items := make([]gen.OidcProvider, 0, len(list))
	for _, p := range list {
		items = append(items, gen.OidcProvider{Id: p.ID, Name: p.Name, Kind: gen.OidcProviderKind(p.Kind)})
	}
	return gen.ListOidcProviders200JSONResponse{Success: true, Message: ptr(messageOidcProviders), Data: &items}, nil
}

func (c *Controller) BeginOidcLogin(ctx context.Context, req gen.BeginOidcLoginRequestObject) (gen.BeginOidcLoginResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	res, err := c.service.BeginOidcLogin(ctx, req.Body.Provider)
	if err != nil {
		return nil, err
	}
	return gen.BeginOidcLogin200JSONResponse{
		Body: authorizationToGen(res), Headers: gen.BeginOidcLogin200ResponseHeaders{SetCookie: oidcCookie(res.CookieValue)},
	}, nil
}

func (c *Controller) CompleteOidcLogin(ctx context.Context, req gen.CompleteOidcLoginRequestObject) (gen.CompleteOidcLoginResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	res, err := c.service.CompleteOidcLogin(ctx, callbackOf(req.Body))
	if err != nil {
		return nil, err
	}
	cleared := auth.ClearedSessionCookie(config.OidcCookieName)
	if res.Pending != nil {
		return cookieResponse{
			status: http.StatusAccepted,
			body:   gen.MfaPendingResponse{Success: true, Message: ptr(messageMfaRequired), Data: pendingToGen(res.Pending)},
			cookies: []string{
				auth.SessionCookie(config.MfaCookieName, res.Pending.CookieValue, config.MfaPendingTTL), cleared,
			},
		}, nil
	}
	return cookieResponse{
		status:  http.StatusOK,
		body:    loginBody(res),
		cookies: []string{auth.SessionCookie(config.SessionCookieName, res.CookieValue, config.SessionAbsoluteTimeout), cleared},
	}, nil
}

func (c *Controller) ListLinkedIdentities(ctx context.Context, _ gen.ListLinkedIdentitiesRequestObject) (gen.ListLinkedIdentitiesResponseObject, error) {
	list, err := c.service.ListIdentities(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]gen.LinkedIdentity, 0, len(list))
	for _, i := range list {
		items = append(items, identityToGen(i))
	}
	return gen.ListLinkedIdentities200JSONResponse{Success: true, Message: ptr(messageIdentities), Data: &items}, nil
}

func (c *Controller) BeginOidcLink(ctx context.Context, req gen.BeginOidcLinkRequestObject) (gen.BeginOidcLinkResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	res, err := c.service.BeginOidcLink(ctx, req.Body.Provider, factorOf(req.Body.CurrentFactor))
	if err != nil {
		return nil, err
	}
	return gen.BeginOidcLink200JSONResponse{
		Body: authorizationToGen(res), Headers: gen.BeginOidcLink200ResponseHeaders{SetCookie: oidcCookie(res.CookieValue)},
	}, nil
}

func (c *Controller) CompleteOidcLink(ctx context.Context, req gen.CompleteOidcLinkRequestObject) (gen.CompleteOidcLinkResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	res, err := c.service.CompleteOidcLink(ctx, callbackOf(req.Body))
	if err != nil {
		return nil, err
	}
	return gen.CompleteOidcLink200JSONResponse{
		Body: gen.LinkedIdentityChangeResponse{
			Success: true, Message: ptr(messageIdentityLinked),
			Data: &gen.LinkedIdentityChange{Identity: identityToGen(res.Identity), Revoked: int(res.Revoked)},
		},
		Headers: gen.CompleteOidcLink200ResponseHeaders{SetCookie: auth.ClearedSessionCookie(config.OidcCookieName)},
	}, nil
}

func (c *Controller) RemoveLinkedIdentity(ctx context.Context, req gen.RemoveLinkedIdentityRequestObject) (gen.RemoveLinkedIdentityResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	n, err := c.service.RemoveIdentity(ctx, req.Id, factorOf(req.Body.CurrentFactor))
	if err != nil {
		return nil, err
	}
	return gen.RemoveLinkedIdentity200JSONResponse{Success: true, Message: ptr(messageIdentityGone), Data: &gen.RevokedCount{Revoked: int(n)}}, nil
}
