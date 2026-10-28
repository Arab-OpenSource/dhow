package tokens

import (
	"context"

	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	gen "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Tokens/Gen"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Tokens/Models"
)

const (
	messageCreated = "tokens.token.created"
	messageListed  = "tokens.list.loaded"
	messageRevoked = "tokens.token.revoked"
)

type Controller struct {
	service *Service
}

var _ gen.StrictServerInterface = (*Controller)(nil)

func NewController(service *Service) *Controller { return &Controller{service: service} }

func ptr[T any](v T) *T { return &v }

func factorOf(f gen.CurrentFactor) auth.Factor {
	out := auth.Factor{Method: string(f.Method)}
	if f.Password != nil {
		out.Password = *f.Password
	}
	if f.Code != nil {
		out.Code = *f.Code
	}
	if f.ChallengeId != nil {
		out.ChallengeID = *f.ChallengeId
	}
	if f.Credential != nil {
		out.Credential = *f.Credential
	}
	return out
}

func toGen(t models.Token) gen.ApiToken {
	scopes := make([]gen.ApiTokenScope, 0, len(t.Scopes))
	for _, s := range t.Scopes {
		scopes = append(scopes, gen.ApiTokenScope(s))
	}
	return gen.ApiToken{
		Id: t.ID, OrganizationId: t.OrganizationID, OrganizationName: t.OrganizationName, Name: t.Name, Scopes: scopes,
		Status: gen.ApiTokenStatus(t.Status), Owner: gen.ApiTokenOwner{Id: t.Owner.ID, Name: t.Owner.Name, Email: t.Owner.Email},
		CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt, LastUsedAt: t.LastUsedAt, LastUsedIp: t.LastUsedIP, RevokedAt: t.RevokedAt,
	}
}

func listToGen(list []models.Token) *[]gen.ApiToken {
	out := make([]gen.ApiToken, 0, len(list))
	for _, t := range list {
		out = append(out, toGen(t))
	}
	return &out
}

func (c *Controller) CreateApiToken(ctx context.Context, req gen.CreateApiTokenRequestObject) (gen.CreateApiTokenResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	scopes := make([]string, 0, len(req.Body.Scopes))
	for _, s := range req.Body.Scopes {
		scopes = append(scopes, string(s))
	}
	res, err := c.service.Create(ctx, req.Id, models.CreateInput{
		Name: req.Body.Name, Scopes: scopes, ExpiresInDays: req.Body.ExpiresInDays,
	}, factorOf(req.Body.CurrentFactor))
	if err != nil {
		return nil, err
	}
	return gen.CreateApiToken200JSONResponse{
		Success: true, Message: ptr(messageCreated), Data: &gen.CreatedApiToken{Token: toGen(res.Token), Value: res.Value},
	}, nil
}

func (c *Controller) ListOrganizationApiTokens(ctx context.Context, req gen.ListOrganizationApiTokensRequestObject) (gen.ListOrganizationApiTokensResponseObject, error) {
	list, err := c.service.ListOrganization(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return gen.ListOrganizationApiTokens200JSONResponse{Success: true, Message: ptr(messageListed), Data: listToGen(list)}, nil
}

func (c *Controller) RevokeOrganizationApiToken(ctx context.Context, req gen.RevokeOrganizationApiTokenRequestObject) (gen.RevokeOrganizationApiTokenResponseObject, error) {
	t, err := c.service.RevokeInOrganization(ctx, req.Id, req.TokenId)
	if err != nil {
		return nil, err
	}
	data := toGen(t)
	return gen.RevokeOrganizationApiToken200JSONResponse{Success: true, Message: ptr(messageRevoked), Data: &data}, nil
}

func (c *Controller) ListMyApiTokens(ctx context.Context, _ gen.ListMyApiTokensRequestObject) (gen.ListMyApiTokensResponseObject, error) {
	list, err := c.service.ListMine(ctx)
	if err != nil {
		return nil, err
	}
	return gen.ListMyApiTokens200JSONResponse{Success: true, Message: ptr(messageListed), Data: listToGen(list)}, nil
}

func (c *Controller) RevokeMyApiToken(ctx context.Context, req gen.RevokeMyApiTokenRequestObject) (gen.RevokeMyApiTokenResponseObject, error) {
	t, err := c.service.RevokeMine(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	data := toGen(t)
	return gen.RevokeMyApiToken200JSONResponse{Success: true, Message: ptr(messageRevoked), Data: &data}, nil
}
