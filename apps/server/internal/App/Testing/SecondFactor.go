package apptest

import (
	"context"
	"crypto/rand"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func (e *Env) SeedPasskey(user uuid.UUID) {
	e.T.Helper()
	credentialID := make([]byte, 32)
	_, err := rand.Read(credentialID)
	require.NoError(e.T, err)
	_, err = e.Pool.Exec(context.Background(),
		`INSERT INTO "Passkeys" ("UserId", "Name", "CredentialId", "PublicKey") VALUES ($1, 'Seeded key', $2, '\x01')`, user, credentialID)
	require.NoError(e.T, err)
}

func (e *Env) SetSecondFactorRule(org uuid.UUID, rule string, since time.Time) {
	e.T.Helper()
	var at *time.Time
	if rule != "off" {
		at = &since
	}
	_, err := e.Pool.Exec(context.Background(),
		`UPDATE "Organizations" SET "RequireSecondFactor" = $2, "RequireSecondFactorSince" = $3 WHERE "Id" = $1`, org, rule, at)
	require.NoError(e.T, err)
}

func (e *Env) SetJoinedAt(org uuid.UUID, at time.Time) {
	e.T.Helper()
	_, err := e.Pool.Exec(context.Background(), `UPDATE "Memberships" SET "CreatedAt" = $2 WHERE "OrganizationId" = $1`, org, at)
	require.NoError(e.T, err)
}

func (c *Client) ChangeSecondFactorRule(org uuid.UUID, rule string, factor map[string]any) Response {
	c.Env.T.Helper()
	return c.Do(Request{Method: http.MethodPut, Path: OrgPath(org, "/second-factor-rule"), Body: map[string]any{
		"RequireSecondFactor": rule, "CurrentFactor": factor,
	}})
}

func (c *Client) SecondFactorDue(org uuid.UUID) *time.Time {
	c.Env.T.Helper()
	for _, m := range c.Me().Memberships {
		if m.OrganizationId == org.String() {
			return m.SecondFactorDue
		}
	}
	return nil
}
