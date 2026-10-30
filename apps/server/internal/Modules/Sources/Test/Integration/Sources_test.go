package integration_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Khaymah/dhow/apps/server/internal/Database/Tx"
	sources "github.com/Khaymah/dhow/apps/server/internal/Modules/Sources"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Sources/Models"
)

const sha = "0123456789abcdef0123456789abcdef01234567"

type fakeProvider struct {
	sources.Provider
	mu      sync.Mutex
	err     error
	reports []sources.StatusReport
}

func (f *fakeProvider) ReportStatus(_ context.Context, _ models.Repository, r sources.StatusReport) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reports = append(f.reports, r)
	return f.err
}

func (f *fakeProvider) failWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *fakeProvider) sent() []sources.StatusReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sources.StatusReport(nil), f.reports...)
}

type setup struct {
	env  *apptest.Env
	org  uuid.UUID
	repo uuid.UUID
}

func linkedRepository(t *testing.T) setup {
	t.Helper()
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	project := env.SeedProject(org, uuid.MustParse(owner.UserID))
	res := owner.Do(apptest.Request{
		Method: http.MethodPut, Path: apptest.OrgPath(org, "/projects/"+project.String()+"/repository"),
		Body: map[string]any{"CloneUrl": "https://git.example.com/team/app.git"},
	})
	require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
	var l struct{ Repository struct{ Id uuid.UUID } }
	res.Decode(t, &l)
	return setup{env: env, org: org, repo: l.Repository.Id}
}

func service(t *testing.T, s setup, providers map[string]sources.Provider) *sources.Service {
	t.Helper()
	svc, err := sources.NewService(sources.Options{
		DB: s.env.Pool, Log: s.env.Wired.Options.Log, Now: s.env.Clock.Now, SecretKey: s.env.Config.SecretKey,
		PublicURL: apptest.PublicOrigin, Providers: providers,
	})
	require.NoError(t, err)
	return svc
}

func record(t *testing.T, s setup, svc *sources.Service, run uuid.UUID, state string) {
	t.Helper()
	require.NoError(t, dbtx.Run(context.Background(), s.env.Pool, func(q *sqlc.Queries) error {
		return svc.RecordStatus(context.Background(), q, sources.DesiredStatus{
			OrganizationID: s.org, RepositoryID: s.repo, RunID: run, Sha: sha, State: state, TargetURL: apptest.PublicOrigin + "/runs/1",
		})
	}))
}

func TestTheGenericProviderNeedsNoCommitStatus(t *testing.T) {
	s := linkedRepository(t)
	svc := service(t, s, nil)
	record(t, s, svc, uuid.New(), sources.StatusRunning)
	pass, err := svc.SendStatuses(context.Background())
	require.NoError(t, err)
	assert.Equal(t, sources.StatusPass{Unneeded: 1}, pass)
	assert.Equal(t, 1, s.env.CountRows(`SELECT count(*) FROM "CommitStatusReports" WHERE "SentState" = 'running'`))
	pass, err = svc.SendStatuses(context.Background())
	require.NoError(t, err)
	assert.Equal(t, sources.StatusPass{}, pass)
}

func TestCommitStatusesRetryWithBackoffAndSendOnlyTheLatestState(t *testing.T) {
	s := linkedRepository(t)
	fake := &fakeProvider{}
	svc := service(t, s, map[string]sources.Provider{models.ProviderGeneric: fake})
	run := uuid.New()
	record(t, s, svc, run, sources.StatusPending)
	fake.failWith(errors.New("the provider is down"))
	pass, err := svc.SendStatuses(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, pass.Failed)

	pass, err = svc.SendStatuses(context.Background())
	require.NoError(t, err)
	assert.Equal(t, sources.StatusPass{}, pass, "the next attempt waits for its backoff")

	record(t, s, svc, run, sources.StatusSuccess)
	fake.failWith(nil)
	s.env.Clock.Advance(time.Minute)
	pass, err = svc.SendStatuses(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, pass.Sent)
	reports := fake.sent()
	require.Len(t, reports, 2)
	assert.Equal(t, sources.StatusSuccess, reports[1].State)
	assert.Equal(t, sha, reports[1].Sha)
}

func TestCommitStatusesAreGivenUpAfterADay(t *testing.T) {
	s := linkedRepository(t)
	fake := &fakeProvider{err: errors.New("still down")}
	svc := service(t, s, map[string]sources.Provider{models.ProviderGeneric: fake})
	record(t, s, svc, uuid.New(), sources.StatusFailed)
	abandoned := 0
	for range 40 {
		pass, err := svc.SendStatuses(context.Background())
		require.NoError(t, err)
		abandoned += pass.Abandoned
		s.env.Clock.Advance(time.Hour)
	}
	assert.Equal(t, 1, abandoned)
	assert.Equal(t, 1, s.env.CountRows(`SELECT count(*) FROM "CommitStatusReports" WHERE "AbandonedAt" IS NOT NULL`))
	assert.LessOrEqual(t, len(fake.sent()), 30)
}

func TestRecordStatusRefusesBadInput(t *testing.T) {
	s := linkedRepository(t)
	svc := service(t, s, nil)
	for _, d := range []sources.DesiredStatus{
		{OrganizationID: s.org, RepositoryID: s.repo, RunID: uuid.New(), Sha: "short", State: sources.StatusPending},
		{OrganizationID: s.org, RepositoryID: s.repo, RunID: uuid.New(), Sha: sha, State: "done"},
	} {
		err := dbtx.Run(context.Background(), s.env.Pool, func(q *sqlc.Queries) error { return svc.RecordStatus(context.Background(), q, d) })
		require.Error(t, err)
	}
	other := s.env.SeedOrganization("Other", "other")
	err := dbtx.Run(context.Background(), s.env.Pool, func(q *sqlc.Queries) error {
		return svc.RecordStatus(context.Background(), q, sources.DesiredStatus{
			OrganizationID: other, RepositoryID: s.repo, RunID: uuid.New(), Sha: sha, State: sources.StatusPending,
		})
	})
	require.Error(t, err, "a status for a repository of another organization breaks the foreign key")
}

func TestDeliveryPartitionsAreMadeAheadAndDroppedAfterThirtyDays(t *testing.T) {
	s := linkedRepository(t)
	ctx := context.Background()
	require.NoError(t, s.env.Wired.Sources.CheckDeliveryPartition(ctx))
	_, err := s.env.Pool.Exec(ctx, `CREATE TABLE "WebhookDeliveries_20200101" (LIKE "WebhookDeliveries" INCLUDING ALL)`)
	require.NoError(t, err)
	_, err = s.env.Pool.Exec(ctx, `ALTER TABLE "WebhookDeliveries" ATTACH PARTITION "WebhookDeliveries_20200101" FOR VALUES FROM ('2020-01-01 00:00:00+00') TO ('2020-01-02 00:00:00+00')`)
	require.NoError(t, err)
	res, err := s.env.Wired.Sources.Maintain(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Dropped)
	assert.Equal(t, 4, s.env.CountRows(`SELECT count(*) FROM pg_inherits WHERE inhparent = '"WebhookDeliveries"'::regclass`))
	_, err = s.env.Pool.Exec(ctx, `SELECT "DropWebhookPartitions"(3)`)
	require.Error(t, err, "deliveries are kept longer than the dedupe window")
}
