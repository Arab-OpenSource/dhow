package audit

import (
	"context"
	"errors"

	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	response "github.com/Khaymah/dhow/apps/server/internal/Core/Response"
	gen "github.com/Khaymah/dhow/apps/server/internal/Modules/Audit/Gen"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Audit/Models"
)

const (
	messageListed   = "audit.events.listed"
	messageVerified = "audit.chain.verified"
	messageLoaded   = "audit.event.loaded"
	fieldCategory   = "Category"
	fieldTo         = "To"
	messageInvalid  = "errors.validationFailed"
)

type Controller struct {
	service *Service
}

var _ gen.StrictServerInterface = (*Controller)(nil)

func NewController(service *Service) *Controller { return &Controller{service: service} }

func (c *Controller) ListAuditEvents(ctx context.Context, req gen.ListAuditEventsRequestObject) (gen.ListAuditEventsResponseObject, error) {
	page, limit := response.PageParams(req.Params.Page, req.Params.Limit)
	filter, err := filterFrom(req.Params)
	if err != nil {
		return nil, err
	}
	events, total, err := c.service.List(ctx, req.Id, filter, page, limit)
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

func (c *Controller) GetAuditEvent(ctx context.Context, req gen.GetAuditEventRequestObject) (gen.GetAuditEventResponseObject, error) {
	event, err := c.service.Get(ctx, req.Id, req.EventId)
	if errors.Is(err, ErrEventNotFound) {
		return nil, apperrors.NotFound()
	}
	if err != nil {
		return nil, apperrors.Internal(err)
	}
	msg, data := messageLoaded, eventToGen(event)
	return gen.GetAuditEvent200JSONResponse{Success: true, Message: &msg, Data: &data}, nil
}

func filterFrom(p gen.ListAuditEventsParams) (models.Filter, error) {
	f := models.Filter{ActorUserID: p.ActorUserId, From: p.From, To: p.To}
	if p.Category != nil {
		if !p.Category.Valid() {
			return models.Filter{}, apperrors.ValidationFailed(apperrors.IErrorDetail{Field: fieldCategory, Message: messageInvalid})
		}
		f.Category = string(*p.Category)
	}
	if p.Action != nil {
		f.Action = *p.Action
	}
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		return models.Filter{}, apperrors.ValidationFailed(apperrors.IErrorDetail{Field: fieldTo, Message: messageInvalid})
	}
	return f, nil
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
