package installer_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"time"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	installer "github.com/Khaymah/dhow/apps/runner/internal/Installer"
	engine "github.com/Khaymah/dhow/internal/installer"
	"github.com/Khaymah/dhow/internal/installer/fakesys"
)

const (
	testToken     = "dhow_rrt_SuperSecretRegistrationToken0123456789"
	testRunnerID  = "0190c8a2-0000-7000-8000-00000000abcd"
	sealedPrefix  = "SEALED:"
	debian13      = "PRETTY_NAME=\"Debian GNU/Linux 13 (trixie)\"\nID=debian\nVERSION_ID=\"13\"\nVERSION_CODENAME=trixie\n"
	ubuntu2604    = "PRETTY_NAME=\"Ubuntu 26.04 LTS\"\nID=ubuntu\nVERSION_ID=\"26.04\"\nVERSION_CODENAME=resolute\n"
	ubuntu2404    = "PRETTY_NAME=\"Ubuntu 24.04 LTS\"\nID=ubuntu\nVERSION_ID=\"24.04\"\nVERSION_CODENAME=noble\n"
	fedora43      = "PRETTY_NAME=\"Fedora Linux 43\"\nID=fedora\nVERSION_ID=43\n"
	rocky10       = "PRETTY_NAME=\"Rocky Linux 10.0\"\nID=\"rocky\"\nVERSION_ID=\"10.0\"\n"
	userLine      = "dhow-runner:x:990:990:Dhow runner (created by dhow-runner install):/var/lib/dhow-runner:/usr/sbin/nologin\n"
	answersPath   = "/root/runner.yaml"
	tokenPath     = "/root/token"
	readyLog      = `{"time":"2026-10-29T12:00:00Z","level":"INFO","msg":"the runner is ready","capacity":1}` + "\n"
	exposureLine  = "→ Overall exposure level for dhow-runner.service: 5.2 MEDIUM 😐\n"
	podmanVersion = "podman version --format {{.Client.Version}}"
)

type fakeProtocol struct {
	mu       sync.Mutex
	sys      *fakesys.Sys
	tokens   []string
	targets  []installer.Target
	revokes  int
	signIns  int
	reaches  int
	revokeFn func() (installer.RevokeResult, error)
	signInFn func() error
}

func (p *fakeProtocol) Register(_ context.Context, t installer.Target, token, _ string) (installer.Registered, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tokens = append(p.tokens, token)
	p.targets = append(p.targets, t)
	path := installer.IdentityPath
	data := `{"SigningSeed":"private"}`
	if t.Encrypted {
		path, data = installer.SealedPath, sealedPrefix+"opaque"
	}
	mode := engine.FileMode{Perm: 0o600, Owner: installer.ServiceUser, Group: installer.ServiceUser}
	if t.Encrypted {
		mode = engine.FileMode{Perm: 0o600}
	}
	if err := p.sys.WriteFile(path, []byte(data), mode); err != nil {
		return installer.Registered{}, err
	}
	return installer.Registered{RunnerID: testRunnerID, Labels: []string{"pool=test"}}, nil
}

func (p *fakeProtocol) Seal(context.Context, installer.Target) error {
	if err := p.sys.WriteFile(installer.SealedPath, []byte(sealedPrefix+"converted"), engine.FileMode{Perm: 0o600}); err != nil {
		return err
	}
	return p.sys.Remove(installer.IdentityPath)
}

func (p *fakeProtocol) SignIn(context.Context, installer.Target) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.signIns++
	if p.signInFn != nil {
		return testRunnerID, p.signInFn()
	}
	return testRunnerID, nil
}

func (p *fakeProtocol) Reach(context.Context, installer.Target) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reaches++
	return testRunnerID, nil
}

func (p *fakeProtocol) Revoke(context.Context, installer.Target) (string, installer.RevokeResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.revokes++
	if p.revokeFn != nil {
		r, err := p.revokeFn()
		return testRunnerID, r, err
	}
	return testRunnerID, installer.RevokeDone, nil
}

func runnerHost(release string) *fakesys.Sys {
	f := fakesys.New()
	f.Put(engine.OSReleasePath, release, 0o644)
	f.PutDir(installer.SystemdRunDir, engine.FileMode{Perm: 0o755})
	for _, b := range []string{"podman", "crun", "newuidmap", "newgidmap", "systemd-creds", "systemd-analyze", "shred"} {
		f.Bins[b] = true
	}
	f.Stdout[podmanVersion] = "5.4.2\n"
	f.Put(installer.CgroupControllers, "cpuset cpu io memory pids\n", 0o444)
	f.Put(installer.MaxUserNamespaces, "63432\n", 0o644)
	f.Put(installer.SubUIDPath, "alice:100000:65536\n", 0o644)
	f.Put(installer.SubGIDPath, "alice:100000:65536\n", 0o644)
	f.PutDir("/root", engine.FileMode{Perm: 0o700})
	f.PutDir("/tmp", engine.FileMode{Perm: fs.ModeSticky | 0o777})
	f.Exe = "/tmp/get/dhow-runner"
	f.Put(f.Exe, "dhow-runner binary", 0o755)
	f.Owners[installer.ServiceUser] = 990
	f.Stdout["systemd-analyze security --no-pager dhow-runner.service"] = exposureLine
	f.OnRun = simulate
	return f
}

func simulate(s *fakesys.Sys, c engine.Command) (engine.Output, bool, error) {
	line := c.String()
	switch {
	case strings.HasPrefix(line, "useradd "):
		s.OK["getent passwd dhow-runner"] = true
		s.Stdout["getent passwd dhow-runner"] = userLine
	case strings.HasPrefix(line, "usermod --add-subuids "):
		span := strings.Split(c.Args[1], "-")
		var start, end int
		_, _ = fmt.Sscan(span[0], &start)
		_, _ = fmt.Sscan(span[1], &end)
		entry := fmt.Sprintf("dhow-runner:%d:%d\n", start, end-start+1)
		for _, p := range []string{installer.SubUIDPath, installer.SubGIDPath} {
			s.Put(p, s.Content(p)+entry, 0o644)
		}
	case strings.HasPrefix(line, "systemd-creds encrypt"):
		if s.Fail["systemd-creds"] {
			return engine.Output{ExitCode: 1}, true, engine.ErrCommandFailed
		}
		return engine.Output{Stdout: append([]byte(sealedPrefix), bytes.ToUpper(c.Stdin)...)}, true, nil
	case line == "systemctl restart dhow-runner":
		s.OK["systemctl is-active --quiet dhow-runner"] = true
		s.OK["systemctl is-enabled --quiet dhow-runner"] = true
		s.Stdout["systemctl show --property InvocationID --value dhow-runner"] = "inv-1\n"
		s.Stdout["journalctl _SYSTEMD_INVOCATION_ID=inv-1 --output cat --no-pager"] = readyLog
	case line == "systemctl disable --now dhow-runner" || line == "systemctl stop dhow-runner":
		s.OK["systemctl is-active --quiet dhow-runner"] = false
	case line == "userdel dhow-runner":
		s.OK["getent passwd dhow-runner"] = false
	}
	return engine.Output{}, false, nil
}

func newApp(f *fakesys.Sys, p *fakeProtocol, in string) (installer.App, *bytes.Buffer) {
	var out bytes.Buffer
	p.sys = f
	return installer.App{
		Sys: f, Protocol: p, In: strings.NewReader(in), InFD: -1, Out: &out, Version: "1.2.3",
		Machine: func() (config.Machine, error) { return config.Machine{}, nil }, Sleep: func(time.Duration) {},
	}, &out
}

func writeAnswers(f *fakesys.Sys, yaml string) {
	f.Put(answersPath, yaml, 0o600)
	f.Put(tokenPath, testToken+"\n", 0o600)
}

const basicAnswers = "url: https://dhow.example.com\ntoken_file: /root/token\nname: build-01\n"

var errTest = errors.New("simulated failure")
