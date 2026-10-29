package integration_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	dbtest "github.com/Khaymah/dhow/apps/server/internal/Database/Testing"
	audit "github.com/Khaymah/dhow/apps/server/internal/Modules/Audit"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Audit/Models"
)

type stepClock struct {
	at time.Time
}

func (c *stepClock) now() time.Time { return c.at }

func newUser(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO "Users" ("Email", "Name", "PasswordHash") VALUES ($1, 'U', 'x') RETURNING "Id"`, email).Scan(&id))
	return id
}

type filterFixture struct {
	service *audit.Service
	org     uuid.UUID
	other   uuid.UUID
	alice   uuid.UUID
	bob     uuid.UUID
	start   time.Time
}

func newFilterFixture(t *testing.T) filterFixture {
	t.Helper()
	pool := dbtest.NewPool(t)
	clock := &stepClock{at: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)}
	f := filterFixture{
		service: audit.NewService(pool, slog.New(slog.NewTextHandler(io.Discard, nil)), clock.now),
		org:     newOrg(t, pool, "acme"),
		other:   newOrg(t, pool, "globex"),
		alice:   newUser(t, pool, "alice@example.com"),
		bob:     newUser(t, pool, "bob@example.com"),
		start:   clock.at,
	}
	steps := []struct {
		action string
		actor  *uuid.UUID
		org    uuid.UUID
	}{
		{"auth.login", &f.alice, f.org},
		{"auth.login_failed", nil, f.org},
		{"identity.team.created", &f.alice, f.org},
		{"runner.registered", &f.bob, f.org},
		{"auth.login", &f.bob, f.org},
		{"auth.login", &f.alice, f.other},
	}
	for _, s := range steps {
		err := f.service.Handle(context.Background(), eventbus.Event{Name: s.action, Payload: eventbus.Audited{
			OrganizationIDs: []uuid.UUID{s.org}, ActorUserID: s.actor, TargetType: "test", TargetID: s.action,
		}})
		require.NoError(t, err)
		clock.at = clock.at.Add(time.Hour)
	}
	return f
}

func actions(events []models.Event) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.Action)
	}
	return out
}

func TestListFilters(t *testing.T) {
	f := newFilterFixture(t)
	at := func(hours int) *time.Time {
		v := f.start.Add(time.Duration(hours) * time.Hour)
		return &v
	}
	tests := []struct {
		name   string
		filter models.Filter
		want   []string
	}{
		{"no filter", models.Filter{}, []string{"auth.login", "runner.registered", "identity.team.created", "auth.login_failed", "auth.login"}},
		{"a category", models.Filter{Category: "auth"}, []string{"auth.login", "auth.login_failed", "auth.login"}},
		{"a category never matches a longer word", models.Filter{Category: "run"}, []string{}},
		{"an exact action", models.Filter{Action: "auth.login"}, []string{"auth.login", "auth.login"}},
		{"an action prefix is not a match", models.Filter{Action: "auth"}, []string{}},
		{"an actor", models.Filter{ActorUserID: &f.bob}, []string{"auth.login", "runner.registered"}},
		{"from is inclusive", models.Filter{From: at(3)}, []string{"auth.login", "runner.registered"}},
		{"to is exclusive", models.Filter{To: at(2)}, []string{"auth.login_failed", "auth.login"}},
		{"a window", models.Filter{From: at(1), To: at(4)}, []string{"runner.registered", "identity.team.created", "auth.login_failed"}},
		{"filters combine", models.Filter{Category: "auth", ActorUserID: &f.alice}, []string{"auth.login"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, total, err := f.service.List(context.Background(), f.org, tt.filter, 1, 100)
			require.NoError(t, err)
			assert.Equal(t, tt.want, actions(events))
			assert.EqualValues(t, len(tt.want), total, "the total counts the matching events only")
		})
	}
}

func TestGetReadsOneEventOfTheOrganizationOnly(t *testing.T) {
	f := newFilterFixture(t)
	mine, _, err := f.service.List(context.Background(), f.org, models.Filter{}, 1, 1)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	got, err := f.service.Get(context.Background(), f.org, mine[0].ID)
	require.NoError(t, err)
	assert.Equal(t, mine[0], got)

	theirs, _, err := f.service.List(context.Background(), f.other, models.Filter{}, 1, 1)
	require.NoError(t, err)
	require.Len(t, theirs, 1)
	_, err = f.service.Get(context.Background(), f.org, theirs[0].ID)
	require.ErrorIs(t, err, audit.ErrEventNotFound, "another organization's event is not found")
	_, err = f.service.Get(context.Background(), f.org, uuid.New())
	require.ErrorIs(t, err, audit.ErrEventNotFound)
}
