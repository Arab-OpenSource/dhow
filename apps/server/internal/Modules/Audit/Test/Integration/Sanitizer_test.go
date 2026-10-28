package integration_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	dbtest "github.com/Khaymah/dhow/apps/server/internal/Database/Testing"
	audit "github.com/Khaymah/dhow/apps/server/internal/Modules/Audit"
	"github.com/Khaymah/dhow/internal/logging"
)

type loggedFixture struct {
	fixture
	logs *bytes.Buffer
}

func newLoggedFixture(t *testing.T) loggedFixture {
	t.Helper()
	pool := dbtest.NewPool(t)
	logs := &bytes.Buffer{}
	svc := audit.NewService(pool, logging.New(logs, slog.LevelDebug), time.Now)
	return loggedFixture{fixture: fixture{pool: pool, service: svc, org: newOrg(t, pool, "acme")}, logs: logs}
}

func marker(t *testing.T) string {
	t.Helper()
	b := make([]byte, 12)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return "MARKER" + hex.EncodeToString(b)
}

func rawEvents(t *testing.T, f fixture) string {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `SELECT "Data"::text FROM "AuditEvents"`)
	require.NoError(t, err)
	defer rows.Close()
	var all strings.Builder
	for rows.Next() {
		var s string
		require.NoError(t, rows.Scan(&s))
		all.WriteString(s)
	}
	require.NoError(t, rows.Err())
	return all.String()
}

func TestRevocationCountsAreRecorded(t *testing.T) {
	f := newLoggedFixture(t)
	require.NoError(t, f.service.Handle(context.Background(), eventbus.Event{Name: eventbus.IdentityMemberMfaReset, Payload: eventbus.Audited{
		OrganizationIDs: []uuid.UUID{f.org}, TargetType: "user", TargetID: "x",
		Data: map[string]string{"RevokedSessions": "3", "RevokedPats": "2", "Passkeys": "1", eventbus.SeverityKey: eventbus.SeverityHigh},
	}}))
	assert.Equal(t, map[string]string{"RevokedSessions": "3", "RevokedPats": "2", "Passkeys": "1", "Severity": "high"},
		storedData(t, f.fixture, eventbus.IdentityMemberMfaReset))
	assert.NotContains(t, f.logs.String(), "audit data field dropped")
}

func TestNoSecretValueReachesTheAuditLog(t *testing.T) {
	f := newLoggedFixture(t)
	secret := marker(t)
	data := map[string]string{"RevokedSessions": "1"}
	for _, k := range []string{
		"Token", "Secret", "Password", "Code", "NewPassword", "CurrentPassword", "SessionToken", "SetupToken", "RecoveryCode",
		"TotpCode", "ClientSecret", "PrivateKey", "ApiKey", "Cookie", "Authorization", "CsrfToken", "Credential", "Value",
		"InviteToken", "token", "password", "Session", "Key",
	} {
		data[k] = secret
	}
	for _, k := range []string{"Name", "Reason", "Email"} {
		data[k] = "dhow_pat_" + secret
	}
	data["Labels"] = "dhow_rat_" + secret
	data["Owner"] = "dhow_rrt_" + secret
	require.NoError(t, f.service.Handle(context.Background(), eventbus.Event{Name: "test.secrets", Payload: eventbus.Audited{
		OrganizationIDs: []uuid.UUID{f.org}, TargetType: "test", TargetID: "x", Data: data,
	}}))

	assert.NotContains(t, rawEvents(t, f.fixture), secret)
	assert.Equal(t, map[string]string{"RevokedSessions": "1"}, storedData(t, f.fixture, "test.secrets"))
	logs := f.logs.String()
	assert.NotContains(t, logs, secret, "the warning names the field, never its value")
	assert.Contains(t, logs, `"field":"Token"`)
	assert.Contains(t, logs, "the value looks like a credential")
}

func TestAnUnknownFieldIsDroppedWithAWarning(t *testing.T) {
	f := newLoggedFixture(t)
	require.NoError(t, f.service.Handle(context.Background(), eventbus.Event{Name: "test.unknown", Payload: eventbus.Audited{
		OrganizationIDs: []uuid.UUID{f.org}, TargetType: "test", TargetID: "x", Data: map[string]string{"Mystery": "x", "Name": "kept"},
	}}))
	assert.Equal(t, map[string]string{"Name": "kept"}, storedData(t, f.fixture, "test.unknown"))
	assert.Contains(t, f.logs.String(), `"field":"Mystery"`)
	assert.Contains(t, f.logs.String(), `"event":"test.unknown"`)
}
