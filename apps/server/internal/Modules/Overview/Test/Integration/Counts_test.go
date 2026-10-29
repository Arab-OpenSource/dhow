package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
)

type fixture struct {
	env     *apptest.Env
	org     uuid.UUID
	other   uuid.UUID
	creator uuid.UUID
	now     time.Time
	q       *sqlc.Queries
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	return fixture{
		env: env, org: env.OrganizationID(d.OrgSlug), other: env.SeedOrganization("Other Org", "other-org"),
		creator: uuid.MustParse(owner.UserID), now: env.Clock.Now(), q: sqlc.New(env.Pool),
	}
}

func at(t time.Time) *time.Time { return &t }

func (f fixture) jobs(t *testing.T) sqlc.CountOverviewJobsRow {
	t.Helper()
	row, err := f.q.CountOverviewJobs(context.Background(), sqlc.CountOverviewJobsParams{
		OrganizationID: f.org, FailedSince: f.now.Add(-config.JobRecentFailureWindow),
	})
	require.NoError(t, err)
	return row
}

func TestJobCountsKeepToTheOrganizationAndSkipDeletedRows(t *testing.T) {
	f := newFixture(t)
	f.env.SeedRunnerWith(f.org, f.creator, apptest.RunnerSeed{})
	f.env.SeedRunnerWith(f.org, f.creator, apptest.RunnerSeed{State: "disabled", Labels: []string{"gpu"}})
	f.env.SeedRunnerWith(f.org, f.creator, apptest.RunnerSeed{Labels: []string{"arm64"}, Deleted: true})
	f.env.SeedRunnerWith(f.other, f.creator, apptest.RunnerSeed{Labels: []string{"arm64"}, Isolation: []string{"container", "gvisor"}})
	recent := at(f.now.Add(-time.Hour))

	seeds := []apptest.JobSeed{
		{State: "running"},
		{State: "leased"},
		{State: "queued"},
		{State: "queued", Labels: []string{"arm64"}},
		{State: "queued", Labels: []string{"gpu"}},
		{State: "queued", Isolation: "gvisor"},
		{State: "failed", FinishedAt: recent},
		{State: "timed_out", FinishedAt: recent},
		{State: "lost", FinishedAt: recent},
		{State: "canceled", FinishedAt: recent},
		{State: "succeeded", FinishedAt: recent},
		{State: "queued", Deleted: true},
		{State: "running", Deleted: true},
		{State: "failed", FinishedAt: recent, Deleted: true},
	}
	for _, s := range seeds {
		f.env.SeedJobWith(f.org, f.creator, s)
	}
	for _, s := range []apptest.JobSeed{{State: "queued"}, {State: "running"}, {State: "failed", FinishedAt: recent}} {
		f.env.SeedJobWith(f.other, f.creator, s)
	}

	assert.Equal(t, sqlc.CountOverviewJobsRow{Running: 2, Queued: 4, QueuedNoRunner: 3, FailedLast24h: 2}, f.jobs(t))
}

func TestFailedJobsCountOnlyInsideTheLast24Hours(t *testing.T) {
	window := config.JobRecentFailureWindow
	tests := []struct {
		name     string
		finished time.Duration
		counted  bool
	}{
		{"one second inside the window", window - time.Second, true},
		{"exactly at the edge", window, false},
		{"one second outside the window", window + time.Second, false},
		{"a minute ago", time.Minute, true},
	}
	f := newFixture(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := f.jobs(t).FailedLast24h
			f.env.SeedJobWith(f.org, f.creator, apptest.JobSeed{State: "timed_out", FinishedAt: at(f.now.Add(-tt.finished))})
			want := before
			if tt.counted {
				want++
			}
			assert.Equal(t, want, f.jobs(t).FailedLast24h)
		})
	}
}

func TestRunnerCountsFollowTheListStatusRule(t *testing.T) {
	f := newFixture(t)
	window := config.RunnerOnlineWindow
	seeds := []apptest.RunnerSeed{
		{LastSeenAt: at(f.now.Add(-time.Minute))},
		{LastSeenAt: at(f.now.Add(-window + time.Second))},
		{LastSeenAt: at(f.now.Add(-window))},
		{LastSeenAt: at(f.now.Add(-time.Hour))},
		{},
		{State: "disabled", LastSeenAt: at(f.now)},
		{State: "revoked", LastSeenAt: at(f.now)},
		{LastSeenAt: at(f.now), Deleted: true},
		{State: "disabled", Deleted: true},
	}
	for _, s := range seeds {
		f.env.SeedRunnerWith(f.org, f.creator, s)
	}
	f.env.SeedRunnerWith(f.other, f.creator, apptest.RunnerSeed{LastSeenAt: at(f.now)})
	f.env.SeedRunnerWith(f.other, f.creator, apptest.RunnerSeed{State: "disabled"})

	row, err := f.q.CountOverviewRunners(context.Background(), sqlc.CountOverviewRunnersParams{
		OrganizationID: f.org, OnlineSince: f.now.Add(-window),
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.CountOverviewRunnersRow{Online: 2, Offline: 3, Paused: 1}, row)
}

func TestPendingInvitationsSkipAcceptedRevokedExpiredAndDeleted(t *testing.T) {
	f := newFixture(t)
	later := f.now.Add(time.Hour)
	for _, s := range []apptest.InvitationSeed{
		{ExpiresAt: later},
		{ExpiresAt: f.now.Add(time.Second)},
		{ExpiresAt: f.now},
		{ExpiresAt: f.now.Add(-time.Hour)},
		{ExpiresAt: later, Accepted: true},
		{ExpiresAt: later, Revoked: true},
		{ExpiresAt: later, Deleted: true},
	} {
		f.env.SeedInvitationWith(f.org, f.creator, s)
	}
	f.env.SeedInvitationWith(f.other, f.creator, apptest.InvitationSeed{ExpiresAt: later})

	pending, err := f.q.CountOverviewInvitations(context.Background(), sqlc.CountOverviewInvitationsParams{OrganizationID: f.org, Now: f.now})
	require.NoError(t, err)
	assert.Equal(t, int64(2), pending)
}
