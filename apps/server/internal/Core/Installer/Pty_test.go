//go:build linux

package installer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

type pty struct {
	master, slave *os.File
	mu            sync.Mutex
	seen          bytes.Buffer
	offset        int
}

func openPty(t *testing.T) *pty {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no pseudo-terminal available: %v", err)
	}
	fd := int(master.Fd())
	require.NoError(t, unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0))
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	require.NoError(t, err)
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR, 0)
	require.NoError(t, err)
	p := &pty{master: master, slave: slave}
	go func() {
		buf := make([]byte, 4096)
		for {
			k, err := master.Read(buf)
			p.mu.Lock()
			p.seen.Write(buf[:k])
			p.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		_ = slave.Close()
		_ = master.Close()
	})
	return p
}

func (p *pty) expect(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		s := p.seen.String()
		p.mu.Unlock()
		if i := strings.Index(s[p.offset:], want); i >= 0 {
			p.offset += i + len(want)
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	t.Fatalf("timed out waiting for %q; terminal shows:\n%s", want, p.seen.String())
}

func (p *pty) send(t *testing.T, line string) {
	t.Helper()
	_, err := p.master.WriteString(line + "\n")
	require.NoError(t, err)
}

func (p *pty) all() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.seen.String()
}

func TestInteractiveInstallThroughAPseudoTerminal(t *testing.T) {
	p := openPty(t)
	f := liveHost(debianHost())
	f.bins["docker"] = true
	f.ok["docker info --format {{.ServerVersion}}"] = true
	f.ok["docker compose version"] = true
	url := "postgres://dhow:Pty-Secret-Pw@db.internal:5432/dhow?sslmode=verify-full"
	app := App{Sys: f, DB: fakeDB{major: 18}, In: p.slave, InFD: int(p.slave.Fd()), Out: p.slave, Version: "1.2.3", Sleep: func(time.Duration) {}}
	require.True(t, IsTerminal(app.InFD))

	done := make(chan error, 1)
	go func() { done <- app.Install(context.Background(), InstallOptions{}) }()

	p.expect(t, "How should Dhow run?")
	p.expect(t, "Number [1]: ")
	p.send(t, "")
	p.expect(t, "Domain that users will open")
	p.send(t, "Dhow.Example.com")
	p.expect(t, "PostgreSQL 18:")
	p.expect(t, "Number [1]: ")
	p.send(t, "2")
	p.expect(t, "hidden while you type: ")
	p.send(t, url)
	p.expect(t, "Connected: PostgreSQL 18, sslmode=verify-full.")
	p.expect(t, "Reverse proxy and TLS:")
	p.expect(t, "Number [1]: ")
	p.send(t, "4")
	p.expect(t, "comma separated")
	p.send(t, "")
	p.expect(t, "Have I Been Pwned")
	p.send(t, "n")
	p.expect(t, "Plan:")
	p.expect(t, "Apply this plan? [y/N]: ")
	p.send(t, "y")

	select {
	case err := <-done:
		require.NoError(t, err, p.all())
	case <-time.After(20 * time.Second):
		t.Fatalf("the installer did not finish:\n%s", p.all())
	}
	p.expect(t, "https://dhow.example.com/setup#TOKEN")
	assert.NotContains(t, p.all(), "Pty-Secret-Pw", "the database URL must not be echoed or printed")
	assert.Equal(t, url+"\n", f.content(DatabaseURLPath))
	a, err := ParseInstalledAnswers(strings.NewReader(f.content(AnswersPath)), true)
	require.NoError(t, err)
	assert.Equal(t, "dhow.example.com", a.Domain)
	assert.Equal(t, ProxyNone, a.Proxy.Kind)
	assert.NotContains(t, f.content(AnswersPath), "Pty-Secret-Pw")

	termios, err := unix.IoctlGetTermios(int(p.slave.Fd()), unix.TCGETS)
	require.NoError(t, err)
	assert.NotZero(t, termios.Lflag&unix.ECHO, "echo must be restored after the secret prompt")
}
