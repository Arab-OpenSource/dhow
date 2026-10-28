package e2e_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

func TestAPasswordChangeRecordsHowManySessionsItEnded(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	_, again := env.Login(d.Email, d.Password)
	require.Equal(t, http.StatusOK, again.Status)
	res := owner.Do(apptest.Request{Method: http.MethodPut, Path: "/me/password", Body: map[string]string{
		"CurrentPassword": d.Password, "NewPassword": "a brand new long password",
	}})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	var revoked string
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		`SELECT "Data"->>'RevokedSessions' FROM "AuditEvents" WHERE "Action" = 'auth.password_changed'`).Scan(&revoked))
	assert.Equal(t, "1", revoked)
	assert.NotContains(t, env.Logs.String(), apptest.AuditFieldDropped)
}
