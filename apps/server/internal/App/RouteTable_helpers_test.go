package app_test

import (
	"context"
	"crypto/rand"
	"net/http"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
)

func mustUUID(v string) uuid.UUID { return uuid.MustParse(v) }

func sessionIDOf(c *apptest.Client) string {
	var me struct{ SessionId string }
	res := c.Do(apptest.Request{Method: http.MethodGet, Path: "/me"})
	res.Decode(c.Env.T, &me)
	return me.SessionId
}

func passkeyIDOf(c *apptest.Client) string {
	credentialID := make([]byte, 32)
	_, err := rand.Read(credentialID)
	require.NoError(c.Env.T, err)
	var id string
	require.NoError(c.Env.T, c.Env.Pool.QueryRow(context.Background(),
		`INSERT INTO "Passkeys" ("UserId", "Name", "CredentialId", "PublicKey") VALUES ($1, 'Route table key', $2, '\x01') RETURNING "Id"::text`,
		c.UserID, credentialID).Scan(&id))
	return id
}

func mfaBodyFor(rule policy.Rule) any {
	wrong := map[string]any{"CurrentFactor": map[string]string{"Method": "password", "Password": "not the password"}}
	switch rule.Method + " " + rule.Path {
	case "POST /sessions/mfa":
		return map[string]string{"Method": "totp", "Code": "000000"}
	case "POST /sessions/passkey":
		return map[string]any{"ChallengeId": "00000000-0000-7000-8000-000000000003", "Credential": map[string]string{}}
	case "POST /me/totp/confirm":
		return map[string]any{"Code": "000000", "CurrentFactor": wrong["CurrentFactor"]}
	case "POST /me/totp/disable", "POST /me/recovery-codes", "POST /me/passkeys/{id}/remove":
		return wrong
	case "POST /me/passkeys":
		return map[string]any{
			"ChallengeId": "00000000-0000-7000-8000-000000000004", "Credential": map[string]string{}, "Name": "Route table key",
			"CurrentFactor": wrong["CurrentFactor"],
		}
	default:
		return nil
	}
}
