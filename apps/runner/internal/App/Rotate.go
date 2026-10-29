package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	client "github.com/Khaymah/dhow/apps/runner/internal/Client"
	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	identity "github.com/Khaymah/dhow/apps/runner/internal/Identity"
	journal "github.com/Khaymah/dhow/apps/runner/internal/Journal"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const CommandRotateKeys = "rotate-keys"

var (
	ErrRunnerRunning = errors.New("the runner is running; stop it first (sudo systemctl stop dhow-runner), then run rotate-keys again")
	ErrJournalBusy   = errors.New("the last run left attempts it has not reported yet; start the runner so it reports them, stop it, then run rotate-keys again")
	ErrUnconfirmed   = errors.New("the server did not confirm the new keys, so the runner keeps its current key; run rotate-keys again to finish (if the server did take them, the runner also finishes the rotation on its next start)")
	ErrNoKeyAccepted = errors.New("the server refuses both the current key and the new key of the unfinished rotation")
	ErrNotSwitched   = errors.New("the server took the new keys, but the identity file was not switched yet; run rotate-keys again or start the runner, and either one finishes it")
)

type Rotation struct {
	Config      config.Config
	Log         *slog.Logger
	Generate    func() (identity.Keys, error)
	SavePending func(path string, id *identity.Identity) error
	Promote     func(pendingPath, path string) error
}

type Rotated struct {
	Identity *identity.Identity
	Resumed  bool
	Finished bool
}

func runRotateKeys(ctx context.Context, args []string, env Env) error {
	fs := newFlags(CommandRotateKeys, env.Stdio.Err)
	dev := fs.Bool("dev", false, "allow plain http:// to localhost, for local development only")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%w: %s", ErrExtraArgs, strings.Join(fs.Args(), " "))
	}
	if err := env.Harden(); err != nil {
		return err
	}
	machine, err := env.Machine()
	if err != nil {
		return err
	}
	cfg, err := config.Load(env.Getenv, config.Options{Dev: *dev, Machine: machine})
	if err != nil {
		return err
	}
	res, err := Rotation{Config: cfg, Log: newLogger(env.Stdio.Err, cfg.LogLevel)}.Run(ctx)
	if err != nil {
		return err
	}
	how := "Rotated"
	if res.Finished {
		how = "Finished an earlier rotation of"
	}
	_, err = fmt.Fprintf(env.Stdio.Out, "%s the keys of runner %s: key version %d. The old key no longer works.\nStart the runner again with: sudo systemctl start dhow-runner\n",
		how, res.Identity.RunnerID, res.Identity.KeyVersion)
	return err
}

func (r Rotation) defaults() Rotation {
	if r.Log == nil {
		r.Log = slog.New(slog.DiscardHandler)
	}
	if r.Generate == nil {
		r.Generate = identity.GenerateKeys
	}
	if r.SavePending == nil {
		r.SavePending = identity.Save
	}
	if r.Promote == nil {
		r.Promote = identity.Promote
	}
	return r
}

func (r Rotation) Run(ctx context.Context) (Rotated, error) {
	r = r.defaults()
	if _, err := identity.Load(r.Config.IdentityPath()); err != nil {
		return Rotated{}, err
	}
	lock, err := config.LockState(r.Config.LockPath())
	if errors.Is(err, config.ErrStateBusy) {
		return Rotated{}, ErrRunnerRunning
	}
	if err != nil {
		return Rotated{}, err
	}
	defer func() { _ = lock.Release() }()
	current, err := identity.Load(r.Config.IdentityPath())
	if err != nil {
		return Rotated{}, err
	}
	if err := identity.RemoveStaleTemps(r.Config.StateDir); err != nil {
		return Rotated{}, err
	}
	j, err := journal.Open(r.Config.JournalPath())
	if err != nil {
		return Rotated{}, err
	}
	if n := len(j.Entries()); n > 0 {
		return Rotated{}, fmt.Errorf("%w (%d attempts)", ErrJournalBusy, n)
	}
	next, resumed, err := r.prepare(current)
	if err != nil {
		return Rotated{}, err
	}
	if resumed {
		done, err := r.finishedEarlier(ctx, next)
		if err != nil || done {
			return Rotated{Identity: next, Resumed: true, Finished: done}, err
		}
	}
	if err := r.confirm(ctx, current, next); err != nil {
		return Rotated{}, err
	}
	return Rotated{Identity: next, Resumed: resumed}, nil
}

func (r Rotation) prepare(current *identity.Identity) (*identity.Identity, bool, error) {
	pending, err := loadPending(r.Config, current)
	if err != nil {
		return nil, false, err
	}
	if pending != nil && !current.SameKeys(pending) {
		next, err := current.Next(pending.Keys)
		if err != nil {
			return nil, false, err
		}
		if pending.KeyVersion != next.KeyVersion {
			if err := r.SavePending(r.Config.PendingIdentityPath(), next); err != nil {
				return nil, false, err
			}
		}
		return next, true, nil
	}
	keys, err := r.Generate()
	if err != nil {
		return nil, false, err
	}
	next, err := current.Next(keys)
	if err != nil {
		return nil, false, err
	}
	if err := r.SavePending(r.Config.PendingIdentityPath(), next); err != nil {
		return nil, false, fmt.Errorf("save the new keys before asking the server to take them: %w", err)
	}
	return next, false, nil
}

func (r Rotation) finishedEarlier(ctx context.Context, next *identity.Identity) (bool, error) {
	c, err := clientFor(r.Config, next, r.Log)
	if err != nil {
		return false, err
	}
	defer c.CloseIdle()
	err = c.Authenticate(ctx)
	switch {
	case err == nil:
		if err := r.Promote(r.Config.PendingIdentityPath(), r.Config.IdentityPath()); err != nil {
			return false, errors.Join(ErrNotSwitched, err)
		}
		return true, nil
	case errors.Is(err, client.ErrCredentials):
		r.Log.Info("resuming an unfinished key rotation with the keys it saved")
		return false, nil
	default:
		return false, fmt.Errorf("check whether the server took the new keys of the unfinished rotation: %w", err)
	}
}

func (r Rotation) confirm(ctx context.Context, current, next *identity.Identity) error {
	c, err := clientFor(r.Config, current, r.Log)
	if err != nil {
		return err
	}
	defer c.CloseIdle()
	proof := current.ProveRotation(next.Keys)
	res, err := c.RotateKeys(ctx, &runnerv1.RotateKeysRequest{
		NewSigningPublicKey: next.SigningPublicKey(), NewEncryptionPublicKey: next.EncryptionPublicKey(),
		OldKeySignature: proof.OldKeySignature, NewKeySignature: proof.NewKeySignature,
	})
	if errors.Is(err, client.ErrCredentials) {
		return errors.Join(ErrNoKeyAccepted, err)
	}
	if err != nil {
		return errors.Join(ErrUnconfirmed, err)
	}
	if v := res.GetKeyVersion(); v != next.KeyVersion {
		r.Log.Warn("the server numbered the new key differently than expected", "expected", next.KeyVersion, "server", v)
		next.KeyVersion = v
		if err := r.SavePending(r.Config.PendingIdentityPath(), next); err != nil {
			return errors.Join(ErrNotSwitched, err)
		}
	}
	if err := r.Promote(r.Config.PendingIdentityPath(), r.Config.IdentityPath()); err != nil {
		return errors.Join(ErrNotSwitched, err)
	}
	return nil
}
