package integration_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
)

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func TestSchemaEnforcesTheRunnerInvariants(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	creator := uuid.MustParse(owner.UserID)
	ctx := context.Background()
	runner := env.SeedRunner(org, creator)

	tests := []struct {
		name string
		sql  string
		args []any
		code string
	}{
		{"uses above max uses", `UPDATE "RunnerRegistrationTokens" SET "Uses" = 2`, nil, "23514"},
		{"more than 50 uses", `UPDATE "RunnerRegistrationTokens" SET "MaxUses" = 51`, nil, "23514"},
		{"longer than a day", `UPDATE "RunnerRegistrationTokens" SET "ExpiresAt" = "CreatedAt" + interval '25 hours'`, nil, "23514"},
		{"a short token hash", `UPDATE "RunnerRegistrationTokens" SET "TokenHash" = '\x00'`, nil, "23514"},
		{"unknown state", `UPDATE "Runners" SET "State" = 'paused' WHERE "Id" = $1`, []any{runner}, "23514"},
		{"revoked without a time", `UPDATE "Runners" SET "State" = 'revoked' WHERE "Id" = $1`, []any{runner}, "23514"},
		{"disabled without a reason", `UPDATE "Runners" SET "State" = 'disabled' WHERE "Id" = $1`, []any{runner}, "23514"},
		{"capacity above 16", `UPDATE "Runners" SET "Capacity" = 17 WHERE "Id" = $1`, []any{runner}, "23514"},
		{"unknown isolation", `UPDATE "Runners" SET "IsolationLevels" = ARRAY['vm'] WHERE "Id" = $1`, []any{runner}, "23514"},
		{"no isolation", `UPDATE "Runners" SET "IsolationLevels" = '{}' WHERE "Id" = $1`, []any{runner}, "23514"},
		{"short signing key", `UPDATE "Runners" SET "SigningPublicKey" = '\x01' WHERE "Id" = $1`, []any{runner}, "23514"},
		{"a second active session", `INSERT INTO "RunnerSessions" ("RunnerId", "TokenHash", "ChallengeNonce", "KeyVersion", "ExpiresAt")
			SELECT $1, sha256(gen_random_uuid()::text::bytea), substring(sha256(gen_random_uuid()::text::bytea) from 1 for 16), 1, now() + interval '1 minute'
			FROM generate_series(1, 2)`, []any{runner}, "23505"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := env.Pool.Exec(ctx, tt.sql, tt.args...)
			require.Error(t, err)
			assert.Equal(t, tt.code, pgCode(err), err.Error())
		})
	}

	var key []byte
	require.NoError(t, env.Pool.QueryRow(ctx, `SELECT "SigningPublicKey" FROM "Runners" WHERE "Id" = $1`, runner).Scan(&key))
	_, err := env.Pool.Exec(ctx, `UPDATE "Runners" SET "IsDeleted" = true WHERE "Id" = $1`, runner)
	require.NoError(t, err)
	other := env.SeedRunner(org, creator)
	_, err = env.Pool.Exec(ctx, `UPDATE "Runners" SET "SigningPublicKey" = $1 WHERE "Id" = $2`, key, other)
	assert.Equal(t, "23505", pgCode(err), "a signing key is never reused, even after a soft delete")
}
