package jobs

import (
	"context"
	"encoding/hex"
	"errors"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Khaymah/dhow/apps/server/internal/Database/Tx"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Jobs/Models"
	"github.com/Khaymah/dhow/internal/jobspec"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const (
	messageInvalidValue = "errors.validationFailed"
	targetJob           = "job"
)

func principalOf(ctx context.Context) (auth.Principal, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return auth.Principal{}, apperrors.Unauthenticated()
	}
	return p, nil
}

func invalid(field string) apperrors.IErrorDetail {
	return apperrors.IErrorDetail{Field: field, Message: messageInvalidValue}
}

func specViolations(err error) error {
	var verr *jobspec.Error
	if !errors.As(err, &verr) {
		return apperrors.ValidationFailed(invalid("Spec"))
	}
	details := make([]apperrors.IErrorDetail, 0, len(verr.Violations))
	for _, v := range verr.Violations {
		details = append(details, invalid("Spec."+v.Field))
	}
	return apperrors.ValidationFailed(details...)
}

func boundedOr(v *int, def, lowest, highest int) (int, bool) {
	if v == nil {
		return def, true
	}
	return *v, *v >= lowest && *v <= highest
}

func (s *Service) Create(ctx context.Context, org uuid.UUID, in models.CreateInput) (models.Detail, error) {
	p, err := principalOf(ctx)
	if err != nil {
		return models.Detail{}, err
	}
	var details []apperrors.IErrorDetail
	if !auth.ValidDisplayName(in.Name, config.JobNameMaxLength) {
		details = append(details, invalid("Name"))
	}
	priority, ok := boundedOr(in.Priority, 0, 0, config.JobPriorityMax)
	if !ok {
		details = append(details, invalid("Priority"))
	}
	retries, ok := boundedOr(in.MaxInfraRetries, config.JobDefaultInfraRetries, 0, config.JobMaxInfraRetries)
	if !ok {
		details = append(details, invalid("MaxInfraRetries"))
	}
	if len(details) > 0 {
		return models.Detail{}, apperrors.ValidationFailed(details...)
	}
	spec, err := jobspec.Prepare(in.Spec)
	if err != nil {
		return models.Detail{}, specViolations(err)
	}
	if names := jobspec.SecretNames(spec); len(names) > 0 {
		if err := s.secrets.Accepts(ctx, org, names); err != nil {
			if errors.Is(err, ErrSecretsUnavailable) {
				return models.Detail{}, apperrors.SecretsUnavailable()
			}
			return models.Detail{}, err
		}
	}
	return s.insert(ctx, p, org, in.Name, spec, priority, retries)
}

func (s *Service) insert(ctx context.Context, p auth.Principal, org uuid.UUID, name string, spec *runnerv1.JobSpec, priority, retries int) (models.Detail, error) {
	raw, err := encodeSpec(spec)
	if err != nil {
		return models.Detail{}, err
	}
	digest, err := jobspec.Digest(spec)
	if err != nil {
		return models.Detail{}, err
	}
	var row sqlc.Job
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if err := q.LockOrganizationQueue(ctx, org.String()); err != nil {
			return err
		}
		queued, err := q.CountQueuedJobs(ctx, org)
		if err != nil {
			return err
		}
		if queued >= config.JobMaxQueuedPerOrg {
			return apperrors.JobQueueFull()
		}
		row, err = q.InsertJob(ctx, sqlc.InsertJobParams{
			OrganizationID: org, CreatedByUserID: p.UserID, Name: name, Spec: raw, SpecDigest: digest, Image: spec.GetImage(),
			Labels: spec.GetLabels(), Isolation: isolationOf(spec), Priority: int32(priority), //nolint:gosec // Bounded to 0..100 above.
			MaxInfraRetries: int32(retries), QueueTimeoutSeconds: config.JobQueueTimeout.Seconds(), //nolint:gosec // Bounded to 0..5 above.
		})
		if err != nil {
			return err
		}
		return q.NotifyJobsQueued(ctx, org.String())
	})
	if err != nil {
		return models.Detail{}, err
	}
	if err := s.publishByUser(ctx, eventbus.JobCreated, p, org, row.Id, map[string]string{
		"Name": row.Name, "Image": spec.GetImage(), "SpecDigest": hex.EncodeToString(digest),
		"Priority": strconv.Itoa(priority), "Steps": strconv.Itoa(len(spec.GetSteps())),
	}); err != nil {
		return models.Detail{}, err
	}
	return models.Detail{Job: jobOf(row), Spec: spec, Attempts: []models.Attempt{}}, nil
}

func (s *Service) List(ctx context.Context, org uuid.UUID, state *string, page, limit int) ([]models.Job, int64, error) {
	q := s.queries()
	filter := pgtype.Text{}
	if state != nil {
		filter = pgtype.Text{String: *state, Valid: true}
	}
	total, err := q.CountJobs(ctx, sqlc.CountJobsParams{OrganizationID: org, State: filter})
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.ListJobs(ctx, sqlc.ListJobsParams{
		OrganizationID: org, State: filter, PageLimit: int32(limit), PageOffset: int32((page - 1) * limit), //nolint:gosec // limit and page are bounded by response.PageParams.
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]models.Job, 0, len(rows))
	for _, r := range rows {
		out = append(out, listedJob(r))
	}
	return out, total, nil
}

func (s *Service) Get(ctx context.Context, org, id uuid.UUID) (models.Detail, error) {
	q := s.queries()
	row, err := q.GetJob(ctx, sqlc.GetJobParams{ID: id, OrganizationID: org})
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Detail{}, apperrors.NotFound()
	}
	if err != nil {
		return models.Detail{}, err
	}
	spec, err := decodeSpec(row.Spec)
	if err != nil {
		return models.Detail{}, err
	}
	attempts, err := q.ListJobAttempts(ctx, sqlc.ListJobAttemptsParams{JobID: id, OrganizationID: org})
	if err != nil {
		return models.Detail{}, err
	}
	steps, err := q.ListJobSteps(ctx, sqlc.ListJobStepsParams{JobID: id, OrganizationID: org})
	if err != nil {
		return models.Detail{}, err
	}
	job := jobOf(row)
	if row.State == models.JobQueued {
		matching, err := q.CountMatchingRunners(ctx, sqlc.CountMatchingRunnersParams{
			OrganizationID: org, Labels: row.Labels, Isolation: row.Isolation,
		})
		if err != nil {
			return models.Detail{}, err
		}
		if matching == 0 {
			job.QueuedReason = models.ReasonNoMatchingRunner
		}
	}
	out := models.Detail{Job: job, Spec: spec, Attempts: make([]models.Attempt, 0, len(attempts))}
	for _, a := range attempts {
		out.Attempts = append(out.Attempts, attemptOf(a, steps))
	}
	return out, nil
}

func (s *Service) Cancel(ctx context.Context, org, id uuid.UUID) (models.Job, error) {
	p, err := principalOf(ctx)
	if err != nil {
		return models.Job{}, err
	}
	var row sqlc.Job
	changed := false
	err = dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		job, err := q.LockJob(ctx, sqlc.LockJobParams{ID: id, OrganizationID: org})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NotFound()
		}
		if err != nil {
			return err
		}
		row = job
		switch {
		case job.State == models.JobQueued:
			row, err = q.CancelQueuedJob(ctx, sqlc.CancelQueuedJobParams{ID: id, UserID: p.UserID})
			changed = err == nil
			return err
		case !models.JobFinished(job.State) && job.CancelRequestedAt == nil:
			row, err = q.RequestJobCancel(ctx, sqlc.RequestJobCancelParams{ID: id, UserID: p.UserID})
			changed = err == nil
			return err
		case job.State == models.JobCanceled || !models.JobFinished(job.State):
			return nil
		default:
			return apperrors.JobFinished()
		}
	})
	if err != nil {
		return models.Job{}, err
	}
	if changed {
		if err := s.publishByUser(ctx, eventbus.JobCancelRequested, p, org, id, map[string]string{"State": row.State}); err != nil {
			return models.Job{}, err
		}
	}
	return jobOf(row), nil
}

func (s *Service) publishByUser(ctx context.Context, name string, p auth.Principal, org, id uuid.UUID, data map[string]string) error {
	actor := p.UserID
	return s.bus.Publish(ctx, eventbus.Event{Name: name, Payload: eventbus.Audited{
		OrganizationIDs: []uuid.UUID{org}, ActorUserID: &actor, TargetType: targetJob, TargetID: id.String(),
		IP: auth.RequestMetaFrom(ctx).IPString(), Data: data,
	}})
}
