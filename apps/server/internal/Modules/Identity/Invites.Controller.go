package identity

import (
	"context"

	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	gen "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Identity/Gen"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Identity/Models"
)

const (
	messageInvitations       = "identity.invitations.listed"
	messageInvitationCreated = "identity.invitation.created"
	messageInvitationRevoked = "identity.invitation.revoked"
	messageInvitationResent  = "identity.invitation.resent"
	messageInvitationLoaded  = "identity.invitation.loaded"
	messageInvitationJoined  = "identity.invitation.accepted"
)

func invitationToGen(i models.Invitation) gen.Invitation {
	return gen.Invitation{
		Id: i.ID, Email: i.Email, Role: gen.BindableRole(i.Role), InvitedByName: i.InvitedByName, CreatedAt: i.CreatedAt,
		ExpiresAt: i.ExpiresAt, SentAt: i.SentAt, SendCount: i.SendCount, EmailStatus: gen.EmailStatus(i.EmailStatus), Expired: i.Expired,
	}
}

func issuedToGen(i models.InvitationIssued) *gen.InvitationIssued {
	return &gen.InvitationIssued{Invitation: invitationToGen(i.Invitation), Link: i.Link}
}

func acceptedToGen(a models.InvitationAccepted) *gen.InvitationAccepted {
	return &gen.InvitationAccepted{
		User:         gen.User{Id: a.User.ID, Email: a.User.Email, Name: a.User.Name, Locale: gen.Locale(a.User.Locale)},
		Organization: orgToGen(a.Organization), Role: gen.Role(a.Role),
	}
}

func (c *Controller) ListInvitations(ctx context.Context, req gen.ListInvitationsRequestObject) (gen.ListInvitationsResponseObject, error) {
	list, err := c.service.ListInvitations(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	items := make([]gen.Invitation, 0, len(list))
	for _, i := range list {
		items = append(items, invitationToGen(i))
	}
	return gen.ListInvitations200JSONResponse{Success: true, Message: ptr(messageInvitations), Data: &items}, nil
}

func (c *Controller) CreateInvitation(ctx context.Context, req gen.CreateInvitationRequestObject) (gen.CreateInvitationResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	issued, err := c.service.CreateInvitation(ctx, req.Id, req.Body.Email, string(req.Body.Role))
	if err != nil {
		return nil, err
	}
	return gen.CreateInvitation201JSONResponse{Success: true, Message: ptr(messageInvitationCreated), Data: issuedToGen(issued)}, nil
}

func (c *Controller) RevokeInvitation(ctx context.Context, req gen.RevokeInvitationRequestObject) (gen.RevokeInvitationResponseObject, error) {
	inv, err := c.service.RevokeInvitation(ctx, req.Id, req.InvitationId)
	if err != nil {
		return nil, err
	}
	data := invitationToGen(inv)
	return gen.RevokeInvitation200JSONResponse{Success: true, Message: ptr(messageInvitationRevoked), Data: &data}, nil
}

func (c *Controller) ResendInvitation(ctx context.Context, req gen.ResendInvitationRequestObject) (gen.ResendInvitationResponseObject, error) {
	issued, err := c.service.ResendInvitation(ctx, req.Id, req.InvitationId)
	if err != nil {
		return nil, err
	}
	return gen.ResendInvitation200JSONResponse{Success: true, Message: ptr(messageInvitationResent), Data: issuedToGen(issued)}, nil
}

func (c *Controller) PreviewInvitation(ctx context.Context, req gen.PreviewInvitationRequestObject) (gen.PreviewInvitationResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	p, err := c.service.PreviewInvitation(ctx, req.Body.Token)
	if err != nil {
		return nil, err
	}
	data := gen.InvitationPreview{
		OrganizationName: p.OrganizationName, InviterName: p.InviterName, Email: p.Email, Role: gen.BindableRole(p.Role),
		ExpiresAt: p.ExpiresAt, AccountExists: p.AccountExists,
	}
	return gen.PreviewInvitation200JSONResponse{Success: true, Message: ptr(messageInvitationLoaded), Data: &data}, nil
}

func (c *Controller) AcceptInvitation(ctx context.Context, req gen.AcceptInvitationRequestObject) (gen.AcceptInvitationResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	in := models.AcceptInput{Token: req.Body.Token, Name: req.Body.Name, Password: req.Body.Password}
	if req.Body.Locale != nil {
		in.Locale = string(*req.Body.Locale)
	}
	accepted, err := c.service.AcceptInvitation(ctx, in)
	if err != nil {
		return nil, err
	}
	return gen.AcceptInvitation200JSONResponse{Success: true, Message: ptr(messageInvitationJoined), Data: acceptedToGen(accepted)}, nil
}

func (c *Controller) AcceptInvitationSignedIn(ctx context.Context, req gen.AcceptInvitationSignedInRequestObject) (gen.AcceptInvitationSignedInResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	accepted, err := c.service.AcceptInvitationSignedIn(ctx, req.Body.Token)
	if err != nil {
		return nil, err
	}
	return gen.AcceptInvitationSignedIn200JSONResponse{Success: true, Message: ptr(messageInvitationJoined), Data: acceptedToGen(accepted)}, nil
}
