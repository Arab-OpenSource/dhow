package e2e_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

const relaySecret = "Relay-Credential-7f3a9c2e-only-here"

type notifications struct {
	mu       sync.Mutex
	payloads []string
}

func (n *notifications) all() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.payloads...)
}

func listen(t *testing.T, env *apptest.Env) *notifications {
	t.Helper()
	conn, err := pgx.ConnectConfig(t.Context(), env.Pool.Config().ConnConfig.Copy())
	require.NoError(t, err)
	_, err = conn.Exec(t.Context(), "LISTEN dhow_settings_changed")
	require.NoError(t, err)
	out := &notifications{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			n, err := conn.WaitForNotification(ctx)
			if err != nil {
				return
			}
			out.mu.Lock()
			out.payloads = append(out.payloads, n.Payload)
			out.mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		cancel()
		<-done
		_ = conn.Close(context.Background())
	})
	return out
}

func TestASavedSecretIsSealedAndNeverReturned(t *testing.T) {
	mail := &apptest.FakeMailer{}
	env := apptest.NewWithOptions(t, apptest.Options{Mailer: mail})
	heard := listen(t, env)
	owner, _, fresh := ownerWithTotp(t, env)
	var responses []string
	saved := func(fields map[string]any) settingsView {
		version := readSettings(t, owner).version("Mail")
		res := put(owner, "mail", version, fresh(), fields)
		require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
		responses = append(responses, string(res.Raw))
		var v settingsView
		res.Decode(t, &v)
		return v
	}

	v := saved(map[string]any{
		"Host": "smtp.example.com", "From": "dhow@example.com", "Username": "dhow", "Password": relaySecret,
	})
	assert.True(t, v.field(t, "Mail", "Password").Set)
	assert.Equal(t, relaySecret, env.Wired.Settings.Current().Values.Mail.Password, "the server can still use it")

	var sealed []byte
	var plain string
	require.NoError(t, env.Pool.QueryRow(t.Context(),
		`SELECT "Secrets", "Value"::text FROM "InstanceSettings" WHERE "Group" = 'mail'`).Scan(&sealed, &plain))
	require.NotEmpty(t, sealed)
	assert.NotContains(t, string(sealed), relaySecret)
	assert.NotContains(t, plain, relaySecret)
	assert.NotContains(t, plain, "Password")

	v = saved(map[string]any{"Port": 2525})
	assert.True(t, v.field(t, "Mail", "Password").Set, "leaving the field out keeps the secret")
	assert.Equal(t, relaySecret, env.Wired.Settings.Current().Values.Mail.Password)

	test := owner.Do(apptest.Request{Method: http.MethodPost, Path: "/instance/settings/mail/test"})
	require.Equal(t, http.StatusOK, test.Status, string(test.Raw))
	responses = append(responses, string(test.Raw), string(owner.Get("/instance/settings").Raw))

	v = saved(map[string]any{"Username": "", "Password": ""})
	assert.False(t, v.field(t, "Mail", "Password").Set, "an empty value clears the secret")
	assert.Empty(t, env.Wired.Settings.Current().Values.Mail.Password)
	require.NoError(t, env.Pool.QueryRow(t.Context(), `SELECT "Secrets" FROM "InstanceSettings" WHERE "Group" = 'mail'`).Scan(&sealed))
	assert.Empty(t, sealed)

	for _, r := range responses {
		assert.NotContains(t, r, relaySecret)
	}
	assert.NotContains(t, env.Logs.String(), relaySecret)
	rows, err := env.Pool.Query(t.Context(), `SELECT "Data"::text FROM "AuditEvents"`)
	require.NoError(t, err)
	audit, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)
	require.NotEmpty(t, audit)
	for _, a := range audit {
		assert.NotContains(t, a, relaySecret)
	}
	require.Eventually(t, func() bool { return len(heard.all()) >= 3 }, 5*time.Second, 20*time.Millisecond)
	for _, p := range heard.all() {
		assert.Equal(t, "mail", p, "a notification names the group only")
	}
	sent := env.SentMail(mail)
	require.NotEmpty(t, sent)
	for _, m := range sent {
		assert.False(t, strings.Contains(m.Subject+m.Body+m.To, relaySecret))
	}
}
