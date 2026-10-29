package settings

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	instancesettings "github.com/Khaymah/dhow/apps/server/internal/Core/Settings"
	gen "github.com/Khaymah/dhow/apps/server/internal/Modules/Settings/Gen"
)

const (
	messageLoaded   = "settings.loaded"
	messageSaved    = "settings.saved"
	messageTestSent = "settings.mail.testSent"
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

func (c *Controller) save(
	ctx context.Context, g settingspec.Group, version int64, list changes, factor gen.CurrentFactor,
) (*gen.InstanceSettings, error) {
	snap, err := c.service.Update(ctx, g, version, []instancesettings.Change(list), factorOf(factor))
	if err != nil {
		return nil, err
	}
	data := settingsToGen(snap)
	return &data, nil
}

func (c *Controller) GetInstanceSettings(ctx context.Context, _ gen.GetInstanceSettingsRequestObject) (gen.GetInstanceSettingsResponseObject, error) {
	snap, err := c.service.Get(ctx)
	if err != nil {
		return nil, err
	}
	data := settingsToGen(snap)
	return gen.GetInstanceSettings200JSONResponse{Success: true, Message: ptr(messageLoaded), Data: &data}, nil
}

func (c *Controller) UpdateMailSettings(ctx context.Context, req gen.UpdateMailSettingsRequestObject) (gen.UpdateMailSettingsResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	data, err := c.save(ctx, settingspec.GroupMail, req.Body.Version, mailChanges(req.Body), req.Body.CurrentFactor)
	if err != nil {
		return nil, err
	}
	return gen.UpdateMailSettings200JSONResponse{Success: true, Message: ptr(messageSaved), Data: data}, nil
}

func (c *Controller) UpdateOrganizationSettings(
	ctx context.Context, req gen.UpdateOrganizationSettingsRequestObject,
) (gen.UpdateOrganizationSettingsResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	data, err := c.save(ctx, settingspec.GroupOrganizations, req.Body.Version, organizationChanges(req.Body), req.Body.CurrentFactor)
	if err != nil {
		return nil, err
	}
	return gen.UpdateOrganizationSettings200JSONResponse{Success: true, Message: ptr(messageSaved), Data: data}, nil
}

func (c *Controller) UpdatePasswordSettings(
	ctx context.Context, req gen.UpdatePasswordSettingsRequestObject,
) (gen.UpdatePasswordSettingsResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	data, err := c.save(ctx, settingspec.GroupPasswords, req.Body.Version, passwordChanges(req.Body), req.Body.CurrentFactor)
	if err != nil {
		return nil, err
	}
	return gen.UpdatePasswordSettings200JSONResponse{Success: true, Message: ptr(messageSaved), Data: data}, nil
}

func (c *Controller) UpdateRateLimitSettings(
	ctx context.Context, req gen.UpdateRateLimitSettingsRequestObject,
) (gen.UpdateRateLimitSettingsResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	data, err := c.save(ctx, settingspec.GroupRateLimits, req.Body.Version, rateLimitChanges(req.Body), req.Body.CurrentFactor)
	if err != nil {
		return nil, err
	}
	return gen.UpdateRateLimitSettings200JSONResponse{Success: true, Message: ptr(messageSaved), Data: data}, nil
}

func (c *Controller) UpdateLogSettings(ctx context.Context, req gen.UpdateLogSettingsRequestObject) (gen.UpdateLogSettingsResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	data, err := c.save(ctx, settingspec.GroupLogs, req.Body.Version, logChanges(req.Body), req.Body.CurrentFactor)
	if err != nil {
		return nil, err
	}
	return gen.UpdateLogSettings200JSONResponse{Success: true, Message: ptr(messageSaved), Data: data}, nil
}

func (c *Controller) SendTestMail(ctx context.Context, _ gen.SendTestMailRequestObject) (gen.SendTestMailResponseObject, error) {
	to, err := c.service.SendTest(ctx)
	if err != nil {
		return nil, err
	}
	data := gen.MailTest{SentTo: openapi_types.Email(to)}
	return gen.SendTestMail200JSONResponse{Success: true, Message: ptr(messageTestSent), Data: &data}, nil
}
