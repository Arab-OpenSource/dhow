package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"

	client "github.com/Khaymah/dhow/apps/runner/internal/Client"
	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	identity "github.com/Khaymah/dhow/apps/runner/internal/Identity"
	"github.com/Khaymah/dhow/internal/buildinfo"
)

type Recovery struct {
	Config  config.Config
	Log     *slog.Logger
	Backoff client.Backoff
	Promote func(pendingPath, path string) error
}

type Recovered struct {
	Identity *identity.Identity
	Client   *client.Client
}

func serverURL(cfg config.Config, id *identity.Identity) (*url.URL, error) {
	if cfg.URL != nil {
		return cfg.URL, nil
	}
	u, err := config.ParseServerURL(id.ServerURL, cfg.Dev)
	if err != nil {
		return nil, fmt.Errorf("the server address saved at registration: %w", err)
	}
	return u, nil
}

func clientFor(cfg config.Config, id *identity.Identity, log *slog.Logger) (*client.Client, error) {
	u, err := serverURL(cfg, id)
	if err != nil {
		return nil, err
	}
	return client.New(client.Options{BaseURL: u, CAFile: cfg.CAFile, Dev: cfg.Dev, Version: buildinfo.Get().Version, Identity: id, Log: log})
}

func loadPending(cfg config.Config, current *identity.Identity) (*identity.Identity, error) {
	path := cfg.PendingIdentityPath()
	exists, err := identity.Exists(path)
	if err != nil || !exists {
		return nil, err
	}
	pending, err := identity.Load(path)
	if err != nil {
		return nil, fmt.Errorf("the new identity of an unfinished key rotation (%s): %w", path, err)
	}
	if err := current.Matches(pending); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return pending, nil
}

func (r Recovery) Run(ctx context.Context, current *identity.Identity) (Recovered, error) {
	if r.Log == nil {
		r.Log = slog.New(slog.DiscardHandler)
	}
	if r.Promote == nil {
		r.Promote = identity.Promote
	}
	if r.Backoff == (client.Backoff{}) {
		r.Backoff = client.DefaultBackoff()
	}
	if err := identity.RemoveStaleTemps(r.Config.StateDir); err != nil {
		return Recovered{}, err
	}
	pending, err := loadPending(r.Config, current)
	if err != nil || pending == nil {
		return Recovered{Identity: current}, err
	}
	if current.SameKeys(pending) {
		if err := os.Remove(r.Config.PendingIdentityPath()); err != nil {
			return Recovered{}, fmt.Errorf("remove the finished pending identity: %w", err)
		}
		return Recovered{Identity: current}, nil
	}
	return r.resolve(ctx, current, pending)
}

func (r Recovery) resolve(ctx context.Context, current, pending *identity.Identity) (Recovered, error) {
	old, err := clientFor(r.Config, current, r.Log)
	if err != nil {
		return Recovered{}, err
	}
	next, err := clientFor(r.Config, pending, r.Log)
	if err != nil {
		return Recovered{}, err
	}
	for n := 0; ; n++ {
		err := old.Authenticate(ctx)
		if err == nil {
			r.Log.Warn("a key rotation did not finish and the server still knows the current key, so the runner keeps it; run dhow-runner rotate-keys again to finish the rotation")
			next.CloseIdle()
			return Recovered{Identity: current, Client: old}, nil
		}
		if errors.Is(err, client.ErrCredentials) {
			err = next.Authenticate(ctx)
			switch {
			case err == nil:
				if err := r.Promote(r.Config.PendingIdentityPath(), r.Config.IdentityPath()); err != nil {
					return Recovered{}, err
				}
				r.Log.Info("finished an interrupted key rotation: the server had taken the new keys", "key_version", pending.KeyVersion)
				old.CloseIdle()
				return Recovered{Identity: pending, Client: next}, nil
			case errors.Is(err, client.ErrCredentials):
				r.Log.Error("the server refuses both the current key and the new key of an unfinished rotation")
				next.CloseIdle()
				return Recovered{Identity: current, Client: old}, nil
			}
		}
		if client.Fatal(err) || !client.Transient(err) {
			return Recovered{}, err
		}
		r.Log.Warn("could not check an unfinished key rotation with the server yet", "code", client.Code(err).String(), "reason", client.Reason(err))
		if err := r.Backoff.Sleep(ctx, n, err); err != nil {
			return Recovered{}, err
		}
	}
}
