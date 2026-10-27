package installer

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prompter(in string) (*Prompter, *bytes.Buffer) {
	var out bytes.Buffer
	return NewPrompter(strings.NewReader(in), &out, -1, NewMessages(LangEn)), &out
}

func TestPrompter(t *testing.T) {
	p, out := prompter("\n  typed  \nbad\nok\n")
	v, err := p.Ask("Name", "default", nil)
	require.NoError(t, err)
	assert.Equal(t, "default", v)
	v, err = p.Ask("Name", "", nil)
	require.NoError(t, err)
	assert.Equal(t, "typed", v)
	v, err = p.Ask("Word", "", func(s string) error {
		if s != "ok" {
			return errors.New("say ok")
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", v)
	assert.Contains(t, out.String(), "say ok")

	p, _ = prompter("9\n2\n")
	choice, err := p.Choose("Pick", []Option{{"a", "A"}, {"b", "B"}}, "a")
	require.NoError(t, err)
	assert.Equal(t, "b", choice)

	for in, want := range map[string]bool{"\n": true, "y\n": true, "YES\n": true, "أيوه\n": true, "لا\n": false, "n\n": false, "maybe\nno\n": false} {
		p, _ = prompter(in)
		got, err := p.Confirm("Go?", true)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	p, _ = prompter("")
	_, err = p.Ask("Name", "", nil)
	require.ErrorIs(t, err, ErrNoInput)

	p, _ = prompter(strings.Repeat("x\n", maxAttempts))
	_, err = p.Confirm("Go?", false)
	require.ErrorIs(t, err, ErrTooManyTries)

	p, _ = prompter("delete dhow data\n")
	match, err := p.Typed("Sure?", PurgePhrase)
	require.NoError(t, err)
	assert.True(t, match)
}

func TestInterviewInArabicWithoutATerminal(t *testing.T) {
	det := Detect(context.Background(), debianHost())
	in := "dhow.example.com\n1\n1\n\n\n"
	var out bytes.Buffer
	m := NewMessages(LangAr)
	iv := Interview{P: NewPrompter(strings.NewReader(in), &out, -1, m), M: m, Det: det, Out: &out}
	a, err := iv.Run(context.Background(), DefaultAnswers())
	require.NoError(t, err, out.String())
	assert.Equal(t, ModeNative, a.Mode)
	assert.Equal(t, DatabaseInstall, a.Database.Mode)
	assert.Equal(t, ProxyCaddy, a.Proxy.Kind)
	assert.True(t, a.BreachedCheck())
	assert.True(t, a.InstallPackages)
	assert.Contains(t, out.String(), "الدومين اللي المستخدمين هيفتحوه")
	assert.Contains(t, out.String(), "postgresql-18 caddy")
	assert.NotContains(t, out.String(), "Docker", "the compose path is offered only when Docker is present")
}

func TestInterviewOffersComposeWithTheDockerWarning(t *testing.T) {
	f := debianHost()
	f.bins["docker"] = true
	f.ok["docker info --format {{.ServerVersion}}"] = true
	f.ok["docker compose version"] = true
	det := Detect(context.Background(), f)
	var out bytes.Buffer
	m := NewMessages(LangEn)
	in := "2\ndhow.example.com\n2\nops@example.com\nn\ny\n"
	iv := Interview{P: NewPrompter(strings.NewReader(in), &out, -1, m), M: m, Det: det, Out: &out}
	a, err := iv.Run(context.Background(), DefaultAnswers())
	require.NoError(t, err, out.String())
	assert.Equal(t, ModeCompose, a.Mode)
	assert.Equal(t, ProxyNginx, a.Proxy.Kind)
	assert.False(t, a.BreachedCheck())
	assert.Contains(t, out.String(), "anyone in the docker group is effectively root")
	assert.Contains(t, out.String(), "1) Native: binary + systemd")
	assert.NotContains(t, out.String(), "PostgreSQL 18:", "compose brings its own database")
}

func TestInterviewRejectsAnOldExistingDatabase(t *testing.T) {
	f := debianHost()
	f.put("/proc/net/tcp", procNetTCP, 0o444)
	f.put("/usr/lib/postgresql/16/bin/postgres", "", 0o755)
	det := Detect(context.Background(), f)
	var out bytes.Buffer
	m := NewMessages(LangEn)
	url := "postgres://dhow:pw@127.0.0.1:5432/dhow?sslmode=require"
	in := "dhow.example.com\n1\n" + url + "\n" + url + "\n" + url + "\n" + url + "\n" + url + "\n"
	iv := Interview{P: NewPrompter(strings.NewReader(in), &out, -1, m), M: m, Det: det, DB: fakeDB{major: 16}, Out: &out}
	_, err := iv.Run(context.Background(), DefaultAnswers())
	require.ErrorIs(t, err, ErrTooManyTries)
	assert.Contains(t, out.String(), "PostgreSQL 16; Dhow needs 18")
	assert.NotContains(t, out.String(), "Install PostgreSQL 18 on this host", "5432 is taken, so installing is not offered")
}
