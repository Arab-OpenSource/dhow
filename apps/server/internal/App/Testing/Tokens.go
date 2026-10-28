package apptest

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type APIToken struct {
	ID    uuid.UUID
	Value string
}

func (e *Env) SeedAPIToken(user, fallbackOrg uuid.UUID, scopes ...string) APIToken {
	e.T.Helper()
	ctx := context.Background()
	if len(scopes) == 0 {
		scopes = []string{"org.read"}
	}
	value, hash, err := e.Wired.Options.APITokens.New()
	require.NoError(e.T, err)
	var membership, org uuid.UUID
	removed := false
	err = e.Pool.QueryRow(ctx, `SELECT "Id", "OrganizationId" FROM "Memberships" WHERE "UserId" = $1 AND NOT "IsDeleted" ORDER BY "CreatedAt" LIMIT 1`, user).Scan(&membership, &org)
	if errors.Is(err, pgx.ErrNoRows) {
		org, removed = fallbackOrg, true
		err = e.Pool.QueryRow(ctx, `INSERT INTO "Memberships" ("UserId", "OrganizationId", "Role") VALUES ($1, $2, 'viewer') RETURNING "Id"`, user, org).Scan(&membership)
	}
	require.NoError(e.T, err)
	var id uuid.UUID
	now := e.Clock.Now()
	require.NoError(e.T, e.Pool.QueryRow(ctx,
		`INSERT INTO "ApiTokens" ("UserId", "OrganizationId", "MembershipId", "Name", "TokenHash", "Scopes", "ExpiresAt", "CreatedAt")
		 VALUES ($1, $2, $3, 'Seeded token', $4, $5, $6, $7) RETURNING "Id"`,
		user, org, membership, hash, scopes, now.Add(30*24*time.Hour), now).Scan(&id))
	if removed {
		_, err = e.Pool.Exec(ctx, `UPDATE "Memberships" SET "IsDeleted" = true WHERE "Id" = $1`, membership)
		require.NoError(e.T, err)
	}
	return APIToken{ID: id, Value: value}
}

func (e *Env) Bearer(token string, req Request) Response {
	e.T.Helper()
	headers := map[string]string{"Authorization": "Bearer " + token}
	for k, v := range req.Headers {
		headers[k] = v
	}
	req.Headers = headers
	return e.Do(req)
}

func (c *Client) CreateAPIToken(org uuid.UUID, name string, scopes []string, days int, factor map[string]any) Response {
	c.Env.T.Helper()
	return c.Do(Request{Method: http.MethodPost, Path: OrgPath(org, "/api-tokens"), Body: map[string]any{
		"Name": name, "Scopes": scopes, "ExpiresInDays": days, "CurrentFactor": factor,
	}})
}

func (c *Client) MustCreateAPIToken(org uuid.UUID, scopes []string, password string) APIToken {
	c.Env.T.Helper()
	res := c.CreateAPIToken(org, "CI", scopes, 30, PasswordFactor(password))
	require.Equal(c.Env.T, http.StatusOK, res.Status, string(res.Raw))
	var data struct {
		Token struct{ Id uuid.UUID }
		Value string
	}
	res.Decode(c.Env.T, &data)
	return APIToken{ID: data.Token.Id, Value: data.Value}
}
