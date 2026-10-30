package apptest

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func (e *Env) SeedProject(org, creator uuid.UUID) uuid.UUID {
	e.T.Helper()
	suffix := make([]byte, 6)
	_, err := rand.Read(suffix)
	require.NoError(e.T, err)
	var id uuid.UUID
	require.NoError(e.T, e.Pool.QueryRow(context.Background(),
		`INSERT INTO "Projects" ("OrganizationId", "Slug", "Name", "CreatedByUserId") VALUES ($1, $2, $3, $4) RETURNING "Id"`,
		org, "p-"+hex.EncodeToString(suffix), "Project "+hex.EncodeToString(suffix), creator).Scan(&id))
	return id
}

func (e *Env) SeedUserProjectBinding(org, project, user uuid.UUID, role string) uuid.UUID {
	e.T.Helper()
	var id uuid.UUID
	require.NoError(e.T, e.Pool.QueryRow(context.Background(),
		`INSERT INTO "RoleBindings" ("OrganizationId", "UserId", "Scope", "ProjectId", "Role") VALUES ($1, $2, 'project', $3, $4) RETURNING "Id"`,
		org, user, project, role).Scan(&id))
	return id
}

func (e *Env) SeedTeamProjectBinding(org, project, user uuid.UUID, role string) uuid.UUID {
	e.T.Helper()
	ctx := context.Background()
	suffix := make([]byte, 6)
	_, err := rand.Read(suffix)
	require.NoError(e.T, err)
	var team, id uuid.UUID
	require.NoError(e.T, e.Pool.QueryRow(ctx, `INSERT INTO "Teams" ("OrganizationId", "Name") VALUES ($1, $2) RETURNING "Id"`,
		org, "Team "+hex.EncodeToString(suffix)).Scan(&team))
	_, err = e.Pool.Exec(ctx, `INSERT INTO "TeamMembers" ("OrganizationId", "TeamId", "UserId") VALUES ($1, $2, $3)`, org, team, user)
	require.NoError(e.T, err)
	require.NoError(e.T, e.Pool.QueryRow(ctx,
		`INSERT INTO "RoleBindings" ("OrganizationId", "TeamId", "Scope", "ProjectId", "Role") VALUES ($1, $2, 'project', $3, $4) RETURNING "Id"`,
		org, team, project, role).Scan(&id))
	return id
}
