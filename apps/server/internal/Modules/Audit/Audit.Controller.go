package audit

import (
	"context"

	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	response "github.com/Khaymah/dhow/apps/server/internal/Core/Response"
	gen "github.com/Khaymah/dhow/apps/server/internal/Modules/Audit/Gen"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Audit/Models"
)

const (
	messageListed   = "audit.events.listed"
	messageVerified = "audit.chain.verified"
)

type Controller struct {
	service *Service
}

var _ gen.StrictServerInterface = (*Controller)(nil)

func NewController(service *Service) *Controller { return &Controller{service: service} }

func (c *Controller) ListAuditEvents(ctx context.Context, req gen.ListAuditEventsRequestObject) (gen.ListAuditEventsResponseObject, error) {
	page, limit := response.PageParams(req.Params.Page, req.Params.Limit)
	events, total, err := c.service.List(ctx, req.Id, page, limit)
	if err != nil {
		return nil, apperrors.Internal(err)
	}
	items := make([]gen.AuditEvent, 0, len(events))
	for _, e := range events {
		items = append(items, eventToGen(e))
	}
	msg := messageListed
	return gen.ListAuditEvents200JSONResponse{
		Success: true, Message: &msg, Data: &items,
		Meta: &gen.PageMeta{Page: page, Limit: limit, Total: int(total)},
	}, nil
}

func (c *Controller) VerifyAuditChain(ctx context.Context, req gen.VerifyAuditChainRequestObject) (gen.VerifyAuditChainResponseObject, error) {
	result, err := c.service.Verify(ctx, req.Id)
	if err != nil {
		return nil, apperrors.Internal(err)
	}
	msg := messageVerified
	data := gen.AuditVerifyResult{Valid: result.Valid, Checked: result.Checked, FirstBrokenId: result.FirstBrokenID}
	return gen.VerifyAuditChain200JSONResponse{Success: true, Message: &msg, Data: &data}, nil
}

func eventToGen(e models.Event) gen.AuditEvent {
	return gen.AuditEvent{
		Id: e.ID, CreatedAt: e.CreatedAt, OrganizationId: e.OrganizationID, ActorUserId: e.ActorUserID,
		Action: e.Action, TargetType: e.TargetType, TargetId: e.TargetID, Ip: e.IP, Data: e.Data,
	}
}
