package jobs

import (
	"context"
	"slices"

	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	response "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Response"
	gen "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Jobs/Gen"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Jobs/Models"
)

const (
	messageListed          = "jobs.list.loaded"
	messageLoaded          = "jobs.job.loaded"
	messageCreated         = "jobs.job.created"
	messageCancelRequested = "jobs.job.cancelRequested"
)

type Controller struct {
	service *Service
}

var _ gen.StrictServerInterface = (*Controller)(nil)

func NewController(service *Service) *Controller { return &Controller{service: service} }

func (c *Controller) ListJobs(ctx context.Context, req gen.ListJobsRequestObject) (gen.ListJobsResponseObject, error) {
	page, limit := response.PageParams(req.Params.Page, req.Params.Limit)
	var state *string
	if req.Params.State != nil {
		v := string(*req.Params.State)
		if !slices.Contains(models.JobStates(), v) {
			return nil, apperrors.ValidationFailed(invalid("State"))
		}
		state = &v
	}
	list, total, err := c.service.List(ctx, req.Id, state, page, limit)
	if err != nil {
		return nil, err
	}
	items := make([]gen.Job, 0, len(list))
	for _, j := range list {
		items = append(items, jobToGen(j))
	}
	return gen.ListJobs200JSONResponse{
		Success: true, Message: ptrOf(messageListed), Data: &items, Meta: &gen.PageMeta{Page: page, Limit: limit, Total: int(total)},
	}, nil
}

func (c *Controller) CreateJob(ctx context.Context, req gen.CreateJobRequestObject) (gen.CreateJobResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	spec, err := specFromGen(req.Body.Spec)
	if err != nil {
		return nil, err
	}
	d, err := c.service.Create(ctx, req.Id, models.CreateInput{
		Name: req.Body.Name, Spec: spec, Priority: req.Body.Priority, MaxInfraRetries: req.Body.MaxInfraRetries,
	})
	if err != nil {
		return nil, err
	}
	data := detailToGen(d)
	return gen.CreateJob201JSONResponse{Success: true, Message: ptrOf(messageCreated), Data: &data}, nil
}

func (c *Controller) GetJob(ctx context.Context, req gen.GetJobRequestObject) (gen.GetJobResponseObject, error) {
	d, err := c.service.Get(ctx, req.Id, req.JobId)
	if err != nil {
		return nil, err
	}
	data := detailToGen(d)
	return gen.GetJob200JSONResponse{Success: true, Message: ptrOf(messageLoaded), Data: &data}, nil
}

func (c *Controller) CancelJob(ctx context.Context, req gen.CancelJobRequestObject) (gen.CancelJobResponseObject, error) {
	j, err := c.service.Cancel(ctx, req.Id, req.JobId)
	if err != nil {
		return nil, err
	}
	data := jobToGen(j)
	return gen.CancelJob200JSONResponse{Success: true, Message: ptrOf(messageCancelRequested), Data: &data}, nil
}
