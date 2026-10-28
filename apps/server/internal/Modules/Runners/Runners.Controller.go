package runners

import (
	"context"

	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	response "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Response"
	gen "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Runners/Gen"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Runners/Models"
)

const (
	messageListed       = "runners.list.loaded"
	messageLoaded       = "runners.runner.loaded"
	messageUpdated      = "runners.runner.updated"
	messageRevoked      = "runners.runner.revoked"
	messageTokensListed = "runners.tokens.listed"
	messageTokenCreated = "runners.token.created"
	messageTokenRevoked = "runners.token.revoked"
)

type Controller struct {
	service *Service
}

var _ gen.StrictServerInterface = (*Controller)(nil)

func NewController(service *Service) *Controller { return &Controller{service: service} }

func (c *Controller) ListRunners(ctx context.Context, req gen.ListRunnersRequestObject) (gen.ListRunnersResponseObject, error) {
	page, limit := response.PageParams(req.Params.Page, req.Params.Limit)
	list, total, err := c.service.List(ctx, req.Id, page, limit)
	if err != nil {
		return nil, err
	}
	items := make([]gen.Runner, 0, len(list))
	for _, r := range list {
		items = append(items, runnerToGen(r))
	}
	return gen.ListRunners200JSONResponse{
		Success: true, Message: ptr(messageListed), Data: &items, Meta: &gen.PageMeta{Page: page, Limit: limit, Total: int(total)},
	}, nil
}

func (c *Controller) GetRunner(ctx context.Context, req gen.GetRunnerRequestObject) (gen.GetRunnerResponseObject, error) {
	r, err := c.service.Get(ctx, req.Id, req.RunnerId)
	if err != nil {
		return nil, err
	}
	data := runnerToGen(r)
	return gen.GetRunner200JSONResponse{Success: true, Message: ptr(messageLoaded), Data: &data}, nil
}

func (c *Controller) UpdateRunner(ctx context.Context, req gen.UpdateRunnerRequestObject) (gen.UpdateRunnerResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	r, err := c.service.Update(ctx, req.Id, req.RunnerId, models.UpdateInput{Name: req.Body.Name, Labels: req.Body.Labels, Disabled: req.Body.Disabled})
	if err != nil {
		return nil, err
	}
	data := runnerToGen(r)
	return gen.UpdateRunner200JSONResponse{Success: true, Message: ptr(messageUpdated), Data: &data}, nil
}

func (c *Controller) RevokeRunner(ctx context.Context, req gen.RevokeRunnerRequestObject) (gen.RevokeRunnerResponseObject, error) {
	r, err := c.service.Revoke(ctx, req.Id, req.RunnerId)
	if err != nil {
		return nil, err
	}
	data := runnerToGen(r)
	return gen.RevokeRunner200JSONResponse{Success: true, Message: ptr(messageRevoked), Data: &data}, nil
}

func (c *Controller) ListRunnerRegistrationTokens(ctx context.Context, req gen.ListRunnerRegistrationTokensRequestObject) (gen.ListRunnerRegistrationTokensResponseObject, error) {
	page, limit := response.PageParams(req.Params.Page, req.Params.Limit)
	list, total, err := c.service.ListRegistrationTokens(ctx, req.Id, page, limit)
	if err != nil {
		return nil, err
	}
	items := make([]gen.RunnerRegistrationToken, 0, len(list))
	for _, t := range list {
		items = append(items, tokenToGen(t))
	}
	return gen.ListRunnerRegistrationTokens200JSONResponse{
		Success: true, Message: ptr(messageTokensListed), Data: &items, Meta: &gen.PageMeta{Page: page, Limit: limit, Total: int(total)},
	}, nil
}

func (c *Controller) CreateRunnerRegistrationToken(ctx context.Context, req gen.CreateRunnerRegistrationTokenRequestObject) (gen.CreateRunnerRegistrationTokenResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	created, err := c.service.CreateRegistrationToken(ctx, req.Id, models.TokenInput{
		Description: req.Body.Description, Labels: req.Body.Labels, MaxUses: req.Body.MaxUses, ExpiresIn: req.Body.ExpiresInSeconds,
	})
	if err != nil {
		return nil, err
	}
	data := gen.CreatedRunnerRegistrationToken{
		RegistrationToken: tokenToGen(created.Token), Token: created.Secret, RegisterCommand: created.RegisterCommand,
	}
	return gen.CreateRunnerRegistrationToken201JSONResponse{Success: true, Message: ptr(messageTokenCreated), Data: &data}, nil
}

func (c *Controller) RevokeRunnerRegistrationToken(ctx context.Context, req gen.RevokeRunnerRegistrationTokenRequestObject) (gen.RevokeRunnerRegistrationTokenResponseObject, error) {
	t, err := c.service.RevokeRegistrationToken(ctx, req.Id, req.TokenId)
	if err != nil {
		return nil, err
	}
	data := tokenToGen(t)
	return gen.RevokeRunnerRegistrationToken200JSONResponse{Success: true, Message: ptr(messageTokenRevoked), Data: &data}, nil
}

func runnerToGen(r models.Runner) gen.Runner {
	levels := make([]gen.RunnerIsolation, 0, len(r.IsolationLevels))
	for _, l := range r.IsolationLevels {
		levels = append(levels, gen.RunnerIsolation(l))
	}
	out := gen.Runner{
		Id: r.ID, OrganizationId: r.OrganizationID, Name: r.Name, Labels: nonNil(r.Labels), CapabilityLabels: nonNil(r.CapabilityLabels),
		Os: r.Os, Arch: r.Arch, Version: r.Version, IsolationLevels: levels, Capacity: r.Capacity,
		State: gen.RunnerState(r.State), Status: gen.RunnerStatus(r.Status), KeyVersion: r.KeyVersion,
		KeysRotatedAt: r.KeysRotatedAt, LastSeenAt: r.LastSeenAt, CreatedAt: r.CreatedAt, RevokedAt: r.RevokedAt,
	}
	if r.DisabledReason != "" {
		reason := gen.RunnerDisabledReason(r.DisabledReason)
		out.DisabledReason = &reason
	}
	return out
}

func tokenToGen(t models.RegistrationToken) gen.RunnerRegistrationToken {
	return gen.RunnerRegistrationToken{
		Id: t.ID, Description: t.Description, Labels: nonNil(t.Labels), MaxUses: t.MaxUses, Uses: t.Uses, ExpiresAt: t.ExpiresAt,
		Status: gen.RunnerRegistrationTokenStatus(t.Status), CreatedAt: t.CreatedAt, CreatedByUserId: t.CreatedByUserID,
		LastUsedAt: t.LastUsedAt, RevokedAt: t.RevokedAt,
	}
}

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func ptr[T any](v T) *T { return &v }
