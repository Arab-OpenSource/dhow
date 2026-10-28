package integration_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

func TestUserBindingsAreUniquePerOrganization(t *testing.T) {
	env := apptest.New(t)
	_, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	other := env.SeedOrganization("Other", "other")
	_, user := env.SignedIn(org, "both@example.com", "viewer")
	ctx := context.Background()
	insert := func(o uuid.UUID) error {
		_, err := env.Pool.Exec(ctx, `INSERT INTO "RoleBindings" ("OrganizationId", "UserId", "Scope", "Role") VALUES ($1, $2, 'org', 'developer')`, o, user)
		return err
	}
	require.NoError(t, insert(org))
	require.NoError(t, insert(other), "the same user can hold a binding in another organization")
	var pgErr *pgconn.PgError
	err := insert(org)
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "UX_RoleBindings_User_Target", pgErr.ConstraintName)
}
