package integration_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Jobs/Models"
)

const checkViolation = "23514"

func pgErr(err error) *pgconn.PgError {
	var e *pgconn.PgError
	if errors.As(err, &e) {
		return e
	}
	return nil
}

type fixture struct {
	env     *apptest.Env
	org     uuid.UUID
	creator uuid.UUID
	runner  uuid.UUID
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	creator := uuid.MustParse(owner.UserID)
	return fixture{env: env, org: org, creator: creator, runner: env.SeedRunner(org, creator)}
}

func path(table map[string][]string, start, target string) []string {
	type node struct {
		state string
		trail []string
	}
	seen := map[string]bool{start: true}
	queue := []node{{start, nil}}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if n.state == target {
			return n.trail
		}
		for _, next := range table[n.state] {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, node{next, append(append([]string{}, n.trail...), next)})
			}
		}
	}
	return nil
}

const (
	setJobState = `UPDATE "Jobs" SET "State" = $2,
		"FinishedAt" = CASE WHEN $2 IN ('succeeded', 'failed', 'canceled', 'timed_out', 'lost') THEN COALESCE("FinishedAt", now()) END
		WHERE "Id" = $1`
	setAttemptState = `UPDATE "JobAttempts" SET "State" = $2, "StartedAt" = COALESCE("StartedAt", now()),
		"FinishedAt" = CASE WHEN $2 IN ('succeeded', 'failed', 'canceled', 'timed_out', 'lost') THEN COALESCE("FinishedAt", now()) END
		WHERE "Id" = $1`
	setLeaseState = `UPDATE "Leases" SET "State" = $2, "EndedAt" = CASE WHEN $2 <> 'active' THEN COALESCE("EndedAt", now()) END WHERE "Id" = $1`
	setStepState  = `UPDATE "AttemptSteps" SET "State" = $2, "FinishedAt" = CASE WHEN $2 <> 'running' THEN COALESCE("FinishedAt", now()) END WHERE "Id" = $1`
)

func (f fixture) newAttempt(t *testing.T) (uuid.UUID, uuid.UUID) {
	t.Helper()
	job := f.env.SeedJob(f.org, f.creator)
	var attempt uuid.UUID
	require.NoError(t, f.env.Pool.QueryRow(context.Background(),
		`INSERT INTO "JobAttempts" ("JobId", "OrganizationId", "RunnerId", "Number") VALUES ($1, $2, $3, 1) RETURNING "Id"`,
		job, f.org, f.runner).Scan(&attempt))
	return job, attempt
}

func (f fixture) newLease(t *testing.T) uuid.UUID {
	t.Helper()
	job, attempt := f.newAttempt(t)
	var lease uuid.UUID
	require.NoError(t, f.env.Pool.QueryRow(context.Background(),
		`INSERT INTO "Leases" ("AttemptId", "JobId", "OrganizationId", "RunnerId", "FencingToken", "GrantDigest", "ExpiresAt")
		 SELECT $1, $2, $3, $4, "FencingToken", sha256('g'), now() + interval '45 seconds' FROM "JobAttempts" WHERE "Id" = $1
		 RETURNING "Id"`, attempt, job, f.org, f.runner).Scan(&lease))
	return lease
}

func (f fixture) newStep(t *testing.T) uuid.UUID {
	t.Helper()
	_, attempt := f.newAttempt(t)
	var step uuid.UUID
	require.NoError(t, f.env.Pool.QueryRow(context.Background(),
		`INSERT INTO "AttemptSteps" ("AttemptId", "Index", "Name") VALUES ($1, 0, 'step') RETURNING "Id"`, attempt).Scan(&step))
	return step
}

func TestTheTriggersEnforceTheStateMachines(t *testing.T) {
	f := newFixture(t)
	machines := []struct {
		name    string
		states  []string
		initial string
		table   map[string][]string
		can     func(string, string) bool
		set     string
		create  func(t *testing.T) uuid.UUID
	}{
		{
			"job", models.JobStates(), models.JobQueued, models.JobTransitions(), models.JobCanMove, setJobState,
			func(*testing.T) uuid.UUID { return f.env.SeedJob(f.org, f.creator) },
		},
		{
			"attempt", models.AttemptStates(), models.AttemptLeased, models.AttemptTransitions(), models.AttemptCanMove, setAttemptState,
			func(t *testing.T) uuid.UUID { _, a := f.newAttempt(t); return a },
		},
		{"lease", models.LeaseStates(), models.LeaseActive, models.LeaseTransitions(), models.LeaseCanMove, setLeaseState, f.newLease},
		{"step", models.StepStates(), models.StepRunning, models.StepTransitions(), models.StepCanMove, setStepState, f.newStep},
	}
	ctx := context.Background()
	for _, m := range machines {
		for _, from := range m.states {
			for _, to := range m.states {
				t.Run(fmt.Sprintf("%s/%s->%s", m.name, from, to), func(t *testing.T) {
					id := m.create(t)
					for _, step := range path(m.table, m.initial, from) {
						_, err := f.env.Pool.Exec(ctx, m.set, id, step)
						require.NoError(t, err, "walking to %s through %s", from, step)
					}
					_, err := f.env.Pool.Exec(ctx, m.set, id, to)
					if m.can(from, to) {
						assert.NoError(t, err)
						return
					}
					require.Error(t, err)
					e := pgErr(err)
					require.NotNil(t, e, err.Error())
					assert.Equal(t, checkViolation, e.Code)
					if len(m.table[from]) > 0 {
						assert.Contains(t, e.Message, "cannot move from "+from+" to "+to)
					} else {
						assert.Contains(t, e.Message, "cannot change", "a finished row is frozen")
					}
				})
			}
		}
	}
}

func TestRowsStartInTheirInitialState(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	job := f.env.SeedJob(f.org, f.creator)
	tests := []struct {
		name string
		sql  string
		args []any
	}{
		{"a job inserted as running", `INSERT INTO "Jobs" ("OrganizationId", "CreatedByUserId", "Name", "Spec", "SpecDigest", "Image", "Isolation",
			"QueueDeadline", "State", "StartedAt") SELECT "OrganizationId", "CreatedByUserId", 'x', "Spec", "SpecDigest", "Image", 'container',
			"QueueDeadline", 'running', now() FROM "Jobs" WHERE "Id" = $1`, []any{job}},
		{"an attempt inserted as running", `INSERT INTO "JobAttempts" ("JobId", "OrganizationId", "RunnerId", "Number", "State", "StartedAt")
			VALUES ($1, $2, $3, 1, 'running', now())`, []any{job, f.org, f.runner}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.env.Pool.Exec(ctx, tt.sql, tt.args...)
			require.Error(t, err)
			assert.Equal(t, checkViolation, pgErr(err).Code)
			assert.Contains(t, pgErr(err).Message, "starts")
		})
	}
}
