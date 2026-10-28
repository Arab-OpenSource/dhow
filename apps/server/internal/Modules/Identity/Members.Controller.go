package identity

import (
	"context"

	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	gen "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Identity/Gen"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Identity/Models"
)

const (
	messageOrgCreated  = "identity.organization.created"
	messageOrgLeft     = "identity.organization.left"
	messageTransferred = "identity.organization.transferred"
	messageRoleChanged = "identity.member.roleChanged"
	messageRemoved     = "identity.member.removed"
	messageMfaReset    = "identity.member.mfaReset"
)

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

func memberChangeToGen(c models.MemberChange) *gen.MemberChange {
	out := &gen.MemberChange{UserId: c.UserID, Revoked: int(c.Revoked)}
	if c.Role != "" {
		out.Role = ptr(gen.Role(c.Role))
	}
	return out
}

func (c *Controller) CreateOrganization(ctx context.Context, req gen.CreateOrganizationRequestObject) (gen.CreateOrganizationResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	org, err := c.service.CreateOrganization(ctx, req.Body.Name, req.Body.Slug)
	if err != nil {
		return nil, err
	}
	data := orgToGen(org)
	return gen.CreateOrganization201JSONResponse{Success: true, Message: ptr(messageOrgCreated), Data: &data}, nil
}

func (c *Controller) LeaveOrganization(ctx context.Context, req gen.LeaveOrganizationRequestObject) (gen.LeaveOrganizationResponseObject, error) {
	if err := c.service.LeaveOrganization(ctx, req.Id); err != nil {
		return nil, err
	}
	return gen.LeaveOrganization200JSONResponse{Success: true, Message: ptr(messageOrgLeft)}, nil
}

func (c *Controller) TransferOwnership(ctx context.Context, req gen.TransferOwnershipRequestObject) (gen.TransferOwnershipResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	change, err := c.service.TransferOwnership(ctx, req.Id, req.Body.UserId, factorOf(req.Body.CurrentFactor))
	if err != nil {
		return nil, err
	}
	return gen.TransferOwnership200JSONResponse{Success: true, Message: ptr(messageTransferred), Data: memberChangeToGen(change)}, nil
}

func (c *Controller) ChangeMemberRole(ctx context.Context, req gen.ChangeMemberRoleRequestObject) (gen.ChangeMemberRoleResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	var factor *auth.Factor
	if req.Body.CurrentFactor != nil {
		f := factorOf(*req.Body.CurrentFactor)
		factor = &f
	}
	change, err := c.service.ChangeRole(ctx, req.Id, req.UserId, string(req.Body.Role), factor)
	if err != nil {
		return nil, err
	}
	return gen.ChangeMemberRole200JSONResponse{Success: true, Message: ptr(messageRoleChanged), Data: memberChangeToGen(change)}, nil
}

func (c *Controller) RemoveMember(ctx context.Context, req gen.RemoveMemberRequestObject) (gen.RemoveMemberResponseObject, error) {
	change, err := c.service.RemoveMember(ctx, req.Id, req.UserId)
	if err != nil {
		return nil, err
	}
	return gen.RemoveMember200JSONResponse{Success: true, Message: ptr(messageRemoved), Data: memberChangeToGen(change)}, nil
}

func (c *Controller) ResetMemberMfa(ctx context.Context, req gen.ResetMemberMfaRequestObject) (gen.ResetMemberMfaResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	change, err := c.service.ResetMfa(ctx, req.Id, req.UserId, factorOf(req.Body.CurrentFactor))
	if err != nil {
		return nil, err
	}
	return gen.ResetMemberMfa200JSONResponse{Success: true, Message: ptr(messageMfaReset), Data: memberChangeToGen(change)}, nil
}
