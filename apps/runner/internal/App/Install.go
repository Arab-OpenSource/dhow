package app

import (
	"context"
	"errors"
	"os"

	client "github.com/Khaymah/dhow/apps/runner/internal/Client"
	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	identity "github.com/Khaymah/dhow/apps/runner/internal/Identity"
	installer "github.com/Khaymah/dhow/apps/runner/internal/Installer"
	"github.com/Khaymah/dhow/internal/buildinfo"
	engine "github.com/Khaymah/dhow/internal/installer"
	"github.com/Khaymah/dhow/internal/runnersig"
)

type protocol struct{ sealer identity.Sealer }

var _ installer.Protocol = protocol{}

func runInstaller(ctx context.Context, args []string, env Env) error {
	if err := refuseTokenFlags(args); err != nil {
		return err
	}
	fd := -1
	if f, ok := env.Stdio.In.(*os.File); ok {
		fd = int(f.Fd())
	}
	locale := env.Getenv("LC_ALL")
	if locale == "" {
		locale = env.Getenv("LANG")
	}
	sealer := env.Sealer
	if sealer == nil {
		sealer = identity.SystemdCreds{}
	}
	return installer.Main(ctx, args, installer.App{
		Sys: engine.NewHost(), Protocol: protocol{sealer: sealer}, In: env.Stdio.In, InFD: fd, Out: env.Stdio.Out,
		Locale: locale, Version: buildinfo.Get().Version, Machine: env.Machine,
	}, env.Stdio.Err)
}

func (p protocol) store(t installer.Target) (identity.Store, error) {
	if t.Encrypted {
		return identity.SealedStore{Dir: t.Config.StateDir, Sealer: p.sealer}, nil
	}
	owner, err := config.OwnerOf(t.Config.StateDir)
	if err != nil {
		return nil, err
	}
	return identity.FileStore{Dir: t.Config.StateDir, Owner: owner}, nil
}

func (p protocol) Register(ctx context.Context, t installer.Target, token, name string) (installer.Registered, error) {
	store, err := p.store(t)
	if err != nil {
		return installer.Registered{}, err
	}
	id, err := RegisterWith(ctx, t.Config, token, RunnerName(name), store)
	if err != nil {
		return installer.Registered{}, err
	}
	return installer.Registered{RunnerID: id.RunnerID, OrganizationID: id.OrganizationID, Labels: id.Labels}, nil
}

func (p protocol) Seal(_ context.Context, t installer.Target) error {
	owner, err := config.OwnerOf(t.Config.StateDir)
	if err != nil {
		return err
	}
	plain := identity.FileStore{Dir: t.Config.StateDir, Owner: owner}
	id, err := plain.Load()
	if err != nil {
		return err
	}
	if err := (identity.SealedStore{Dir: t.Config.StateDir, Sealer: p.sealer}).Save(id); err != nil {
		return err
	}
	return config.ShredFile(plain.Path())
}

func (p protocol) load(t installer.Target) (*identity.Identity, error) {
	store, err := p.store(t)
	if err != nil {
		return nil, err
	}
	return store.Load()
}

func (p protocol) client(t installer.Target) (*client.Client, *identity.Identity, error) {
	id, err := p.load(t)
	if err != nil {
		return nil, nil, err
	}
	c, err := clientFor(t.Config, id, nil)
	if err != nil {
		return nil, id, err
	}
	return c, id, nil
}

func (p protocol) SignIn(ctx context.Context, t installer.Target) (string, error) {
	c, id, err := p.client(t)
	if err != nil {
		return "", err
	}
	defer c.CloseIdle()
	return id.RunnerID, c.Authenticate(ctx)
}

func (p protocol) Reach(ctx context.Context, t installer.Target) (string, error) {
	c, id, err := p.client(t)
	if err != nil {
		return "", err
	}
	defer c.CloseIdle()
	return id.RunnerID, c.Reach(ctx)
}

func (p protocol) Revoke(ctx context.Context, t installer.Target) (string, installer.RevokeResult, error) {
	c, id, err := p.client(t)
	if err != nil {
		runnerID := ""
		if id != nil {
			runnerID = id.RunnerID
		}
		return runnerID, installer.RevokeDone, err
	}
	defer c.CloseIdle()
	err = c.Revoke(ctx, id.Sign(runnersig.RevokeMessage(id.ServerOrigin, id.RunnerID, id.KeyVersion)))
	if errors.Is(err, client.ErrRevoked) {
		return id.RunnerID, installer.RevokeAlready, nil
	}
	return id.RunnerID, installer.RevokeDone, err
}
