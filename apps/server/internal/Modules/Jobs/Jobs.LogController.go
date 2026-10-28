package jobs

import (
	"context"
	"net/http"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	gen "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Jobs/Gen"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Jobs/Models"
)

const messageLogsLoaded = "jobs.logs.loaded"

func attemptParam(v *int) (*int, error) {
	if v != nil && *v < 1 {
		return nil, apperrors.ValidationFailed(invalid("Attempt"))
	}
	return v, nil
}

func (c *Controller) GetJobLogs(ctx context.Context, req gen.GetJobLogsRequestObject) (gen.GetJobLogsResponseObject, error) {
	attempt, err := attemptParam(req.Params.Attempt)
	if err != nil {
		return nil, err
	}
	in := models.LogQuery{Attempt: attempt, FromSeq: 1, Limit: config.LogPageDefaultLimit}
	if v := req.Params.FromSeq; v != nil {
		if *v < 1 {
			return nil, apperrors.ValidationFailed(invalid("FromSeq"))
		}
		in.FromSeq = *v
	}
	if v := req.Params.Limit; v != nil {
		if *v < 1 || *v > config.LogPageMaxLimit {
			return nil, apperrors.ValidationFailed(invalid("Limit"))
		}
		in.Limit = *v
	}
	page, err := c.service.Logs(ctx, req.Id, req.JobId, in)
	if err != nil {
		return nil, err
	}
	data := logPageToGen(page)
	return gen.GetJobLogs200JSONResponse{Success: true, Message: ptrOf(messageLogsLoaded), Data: &data}, nil
}

func (c *Controller) StreamJobLogs(ctx context.Context, req gen.StreamJobLogsRequestObject) (gen.StreamJobLogsResponseObject, error) {
	p, err := principalOf(ctx)
	if err != nil {
		return nil, err
	}
	attempt, err := attemptParam(req.Params.Attempt)
	if err != nil {
		return nil, err
	}
	after := int64(0)
	switch {
	case req.Params.LastEventID != nil:
		if *req.Params.LastEventID < 0 {
			return nil, apperrors.ValidationFailed(invalid("Last-Event-ID"))
		}
		after = *req.Params.LastEventID
	case req.Params.FromSeq != nil:
		if *req.Params.FromSeq < 1 {
			return nil, apperrors.ValidationFailed(invalid("FromSeq"))
		}
		after = *req.Params.FromSeq - 1
	}
	org := req.Id
	stream, err := c.service.OpenLogStream(ctx, StreamRequest{
		Organization: org, Job: req.JobId, Attempt: attempt, After: after, User: p.UserID,
		Reauthorize: func(ctx context.Context) error { return c.guard.Reauthorize(ctx, policy.LogRead, org) },
	})
	if err != nil {
		return nil, err
	}
	return streamResponse{stream: stream}, nil
}

type streamResponse struct {
	stream *LogStream
}

func (r streamResponse) VisitStreamJobLogsResponse(w http.ResponseWriter) error {
	return r.stream.Serve(w)
}

func logPageToGen(p models.LogPage) gen.JobLogPage {
	chunks := make([]gen.JobLogChunk, 0, len(p.Chunks))
	for _, c := range p.Chunks {
		chunks = append(chunks, gen.JobLogChunk{
			Seq: c.Seq, Stream: gen.JobLogStream(c.Stream), StepIndex: c.StepIndex, EmittedAt: c.EmittedAt, CreatedAt: c.CreatedAt, Text: c.Text,
		})
	}
	return gen.JobLogPage{
		Attempt: p.Attempt, AttemptId: p.AttemptID, State: gen.JobAttemptState(p.State), LastSeq: p.LastSeq,
		Truncated: p.Truncated, NextSeq: p.NextSeq, Chunks: chunks,
	}
}
