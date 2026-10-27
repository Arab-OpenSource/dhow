package integration_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	dbtest "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Testing"
	audit "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Audit"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Audit/Models"
)

type fixture struct {
	pool    *pgxpool.Pool
	service *audit.Service
	org     uuid.UUID
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := dbtest.NewPool(t)
	clock := time.Now
	svc := audit.NewService(pool, slog.New(slog.NewTextHandler(io.Discard, nil)), clock)
	return fixture{pool: pool, service: svc, org: newOrg(t, pool, "acme")}
}

func newOrg(t *testing.T, pool *pgxpool.Pool, slug string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO "Organizations" ("Name", "Slug") VALUES ($1, $1) RETURNING "Id"`, slug).Scan(&id))
	return id
}

func (f fixture) record(t *testing.T, action string, orgs ...uuid.UUID) {
	t.Helper()
	err := f.service.Handle(context.Background(), eventbus.Event{Name: action, Payload: eventbus.Audited{
		OrganizationIDs: orgs, TargetType: "test", TargetID: action, IP: "203.0.113.7", Data: map[string]string{"n": action},
	}})
	require.NoError(t, err)
}

func (f fixture) ids(t *testing.T, org uuid.UUID) []uuid.UUID {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `SELECT "Id" FROM "AuditEvents" WHERE "OrganizationId" = $1 ORDER BY "Seq"`, org)
	require.NoError(t, err)
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		require.NoError(t, rows.Scan(&id))
		out = append(out, id)
	}
	require.NoError(t, rows.Err())
	return out
}

func (f fixture) tamper(t *testing.T, statement string, args ...any) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `ALTER TABLE "AuditEvents" DISABLE TRIGGER "TR_AuditEvents_AppendOnly"`)
	require.NoError(t, err, "tampering needs the table owner")
	_, err = tx.Exec(ctx, statement, args...)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
}

func TestAppendBuildsALinkedChain(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	empty, err := f.service.Verify(ctx, f.org)
	require.NoError(t, err)
	assert.Equal(t, models.VerifyResult{Valid: true}, empty)

	for i := range 4 {
		f.record(t, fmt.Sprintf("test.event%d", i), f.org)
	}
	res, err := f.service.Verify(ctx, f.org)
	require.NoError(t, err)
	assert.True(t, res.Valid)
	assert.Equal(t, 4, res.Checked)
	assert.Nil(t, res.FirstBrokenID)

	rows, err := f.pool.Query(ctx, `SELECT "PrevHash", "Hash" FROM "AuditEvents" WHERE "OrganizationId" = $1 ORDER BY "Seq"`, f.org)
	require.NoError(t, err)
	defer rows.Close()
	prev := models.GenesisHash()
	for rows.Next() {
		var p, h []byte
		require.NoError(t, rows.Scan(&p, &h))
		assert.Equal(t, prev, p, "each event points at the hash of the one before")
		prev = h
	}
	require.NoError(t, rows.Err())
}

func TestConcurrentWritersNeverForkTheChain(t *testing.T) {
	f := newFixture(t)
	const writers, perWriter = 8, 12
	var wg sync.WaitGroup
	errs := make(chan error, writers*perWriter)
	for w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perWriter {
				err := f.service.Handle(context.Background(), eventbus.Event{Name: "test.concurrent", Payload: eventbus.Audited{
					OrganizationIDs: []uuid.UUID{f.org}, TargetType: "writer", TargetID: fmt.Sprintf("%d-%d", w, i),
				}})
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	ctx := context.Background()
	res, err := f.service.Verify(ctx, f.org)
	require.NoError(t, err)
	assert.True(t, res.Valid, "the chain must verify after concurrent writes")
	assert.Equal(t, writers*perWriter, res.Checked)

	var total, distinctPrev, distinctHash int
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT count(*), count(DISTINCT "PrevHash"), count(DISTINCT "Hash") FROM "AuditEvents" WHERE "OrganizationId" = $1`, f.org).
		Scan(&total, &distinctPrev, &distinctHash))
	assert.Equal(t, writers*perWriter, total)
	assert.Equal(t, total, distinctPrev, "two events sharing a PrevHash would be a fork")
	assert.Equal(t, total, distinctHash)
}

func TestConcurrentWritersOnDifferentChainsDoNotBlockOrMix(t *testing.T) {
	f := newFixture(t)
	other := newOrg(t, f.pool, "globex")
	var wg sync.WaitGroup
	for _, org := range []*uuid.UUID{&f.org, &other, nil} {
		for range 3 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var orgs []uuid.UUID
				if org != nil {
					orgs = []uuid.UUID{*org}
				}
				assert.NoError(t, f.service.Handle(context.Background(), eventbus.Event{Name: "test.mixed", Payload: eventbus.Audited{OrganizationIDs: orgs}}))
			}()
		}
	}
	wg.Wait()
	for _, org := range []uuid.UUID{f.org, other} {
		res, err := f.service.Verify(context.Background(), org)
		require.NoError(t, err)
		assert.True(t, res.Valid)
		assert.Equal(t, 3, res.Checked)
	}
	var instance int
	require.NoError(t, f.pool.QueryRow(context.Background(), `SELECT count(*) FROM "AuditEvents" WHERE "OrganizationId" IS NULL`).Scan(&instance))
	assert.Equal(t, 3, instance)
}

func TestAnEventForSeveralOrganizationsWritesOneRowEach(t *testing.T) {
	f := newFixture(t)
	other := newOrg(t, f.pool, "globex")
	f.record(t, "auth.login", f.org, other)
	assert.Len(t, f.ids(t, f.org), 1)
	assert.Len(t, f.ids(t, other), 1)
}

func TestTamperingIsDetected(t *testing.T) {
	tests := []struct {
		name      string
		tamper    func(t *testing.T, f fixture, ids []uuid.UUID)
		wantBreak int
		wantCount int
	}{
		{
			name: "an edited field",
			tamper: func(t *testing.T, f fixture, ids []uuid.UUID) {
				f.tamper(t, `UPDATE "AuditEvents" SET "Action" = 'auth.nothing_happened' WHERE "Id" = $1`, ids[2])
			},
			wantBreak: 2, wantCount: 3,
		},
		{
			name: "an edited actor address",
			tamper: func(t *testing.T, f fixture, ids []uuid.UUID) {
				f.tamper(t, `UPDATE "AuditEvents" SET "Ip" = '198.51.100.99' WHERE "Id" = $1`, ids[0])
			},
			wantBreak: 0, wantCount: 1,
		},
		{
			name: "edited data",
			tamper: func(t *testing.T, f fixture, ids []uuid.UUID) {
				f.tamper(t, `UPDATE "AuditEvents" SET "Data" = '{"n":"forged"}'::jsonb WHERE "Id" = $1`, ids[4])
			},
			wantBreak: 4, wantCount: 5,
		},
		{
			name: "a replaced hash",
			tamper: func(t *testing.T, f fixture, ids []uuid.UUID) {
				f.tamper(t, `UPDATE "AuditEvents" SET "Hash" = sha256('forged') WHERE "Id" = $1`, ids[1])
			},
			wantBreak: 1, wantCount: 2,
		},
		{
			name: "a deleted event in the middle",
			tamper: func(t *testing.T, f fixture, ids []uuid.UUID) {
				f.tamper(t, `DELETE FROM "AuditEvents" WHERE "Id" = $1`, ids[2])
			},
			wantBreak: 3, wantCount: 3,
		},
		{
			name: "a deleted first event",
			tamper: func(t *testing.T, f fixture, ids []uuid.UUID) {
				f.tamper(t, `DELETE FROM "AuditEvents" WHERE "Id" = $1`, ids[0])
			},
			wantBreak: 1, wantCount: 1,
		},
		{
			name: "a hash that does not match the content",
			tamper: func(t *testing.T, f fixture, ids []uuid.UUID) {
				f.tamper(t, `UPDATE "AuditEvents" SET "Hash" = sha256('x'::bytea) WHERE "Id" = $1`, ids[3])
			},
			wantBreak: 3, wantCount: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			for i := range 5 {
				f.record(t, fmt.Sprintf("test.event%d", i), f.org)
			}
			ids := f.ids(t, f.org)
			require.Len(t, ids, 5)
			tt.tamper(t, f, ids)
			res, err := f.service.Verify(context.Background(), f.org)
			require.NoError(t, err)
			assert.False(t, res.Valid)
			require.NotNil(t, res.FirstBrokenID)
			assert.Equal(t, ids[tt.wantBreak], *res.FirstBrokenID)
			assert.Equal(t, tt.wantCount, res.Checked)
		})
	}
}

func TestTheDatabaseRefusesToChangeHistory(t *testing.T) {
	f := newFixture(t)
	f.record(t, "test.event", f.org)
	ctx := context.Background()
	tests := []struct {
		name      string
		statement string
	}{
		{"update", `UPDATE "AuditEvents" SET "Action" = 'x'`},
		{"update of the soft delete flag", `UPDATE "AuditEvents" SET "IsDeleted" = true`},
		{"delete", `DELETE FROM "AuditEvents"`},
		{"truncate", `TRUNCATE "AuditEvents"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.pool.Exec(ctx, tt.statement)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "append-only")
		})
	}
	assert.Len(t, f.ids(t, f.org), 1)
	_, err := f.pool.Exec(ctx, `INSERT INTO "AuditEvents" ("Action", "PrevHash", "Hash") VALUES ('test.insert', sha256('a'), sha256('b'))`)
	assert.NoError(t, err, "appending stays possible")
}

func TestSecretLookingKeysNeverReachTheLog(t *testing.T) {
	f := newFixture(t)
	err := f.service.Handle(context.Background(), eventbus.Event{Name: "auth.login", Payload: eventbus.Audited{
		OrganizationIDs: []uuid.UUID{f.org}, TargetType: "session",
		Data: map[string]string{
			"Password": "hunter2hunter2", "NewPassword": "x", "csrf_token": "abc", "Authorization": "Bearer abc",
			"SessionCookie": "abc", "ApiKey": "k", "Reason": "ok", "EmailDigest": "d41d8cd9",
		},
	}})
	require.NoError(t, err)
	events, total, err := f.service.List(context.Background(), f.org, 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	assert.Equal(t, map[string]string{"Reason": "ok", "EmailDigest": "d41d8cd9"}, events[0].Data)
}

func TestHandleRejectsAWrongPayload(t *testing.T) {
	f := newFixture(t)
	err := f.service.Handle(context.Background(), eventbus.Event{Name: "auth.login", Payload: "not an audit payload"})
	assert.Error(t, err)
}

func TestListIsNewestFirstAndPaged(t *testing.T) {
	f := newFixture(t)
	other := newOrg(t, f.pool, "globex")
	for i := range 7 {
		f.record(t, fmt.Sprintf("test.event%d", i), f.org)
	}
	f.record(t, "test.other", other)

	first, total, err := f.service.List(context.Background(), f.org, 1, 3)
	require.NoError(t, err)
	assert.EqualValues(t, 7, total, "the count only covers this organization")
	require.Len(t, first, 3)
	assert.Equal(t, "test.event6", first[0].Action)
	last, _, err := f.service.List(context.Background(), f.org, 3, 3)
	require.NoError(t, err)
	require.Len(t, last, 1)
	assert.Equal(t, "test.event0", last[0].Action)
	beyond, _, err := f.service.List(context.Background(), f.org, 9, 3)
	require.NoError(t, err)
	assert.Empty(t, beyond)
	for _, e := range first {
		assert.Equal(t, f.org, *e.OrganizationID)
	}
}

func TestVerifyWalksPastTheBatchSize(t *testing.T) {
	if testing.Short() {
		t.Skip("writes more than one verification batch")
	}
	f := newFixture(t)
	const n = 520
	for i := range n {
		f.record(t, fmt.Sprintf("test.bulk%d", i), f.org)
	}
	res, err := f.service.Verify(context.Background(), f.org)
	require.NoError(t, err)
	assert.True(t, res.Valid)
	assert.Equal(t, n, res.Checked)

	ids := f.ids(t, f.org)
	f.tamper(t, `UPDATE "AuditEvents" SET "TargetId" = 'forged' WHERE "Id" = $1`, ids[505])
	res, err = f.service.Verify(context.Background(), f.org)
	require.NoError(t, err)
	assert.False(t, res.Valid)
	assert.Equal(t, ids[505], *res.FirstBrokenID, "a break in the second batch is found")
	assert.Equal(t, 506, res.Checked)
}
