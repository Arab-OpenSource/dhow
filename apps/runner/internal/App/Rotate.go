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
	Store       identity.Store
	Generate    func() (identity.Keys, error)
	SavePending func(path string, id *identity.Identity) error
	Promote     func(pendingPath, path string, next *identity.Identity) error
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
	getenv := env.Getenv
	if env.Root() {
		if getenv, err = config.WithEnvFile(env.Getenv, config.EnvFilePath); err != nil {
			return err
		}
	}
	cfg, err := config.Load(getenv, config.Options{Dev: *dev, Machine: machine, Privileged: env.Root()})
	if err != nil {
		return err
	}
	store, err := openStore(cfg, env)
	if err != nil {
		return err
	}
	res, err := Rotation{Config: cfg, Log: newLogger(env.Stdio.Err, cfg.LogLevel), Store: store}.Run(ctx)
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
	if r.Store == nil {
		r.Store = identity.NewFileStore(r.Config.StateDir)
	}
	store := r.Store
	if r.SavePending == nil {
		r.SavePending = func(_ string, id *identity.Identity) error { return store.SavePending(id) }
	}
	if r.Promote == nil {
		r.Promote = func(_, _ string, next *identity.Identity) error { return store.Promote(next) }
	}
	return r
}

func (r Rotation) Run(ctx context.Context) (Rotated, error) {
	r = r.defaults()
	if _, err := r.Store.Load(); err != nil {
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
	current, err := r.Store.Load()
	if err != nil {
		return Rotated{}, err
	}
	if err := r.Store.RemoveStaleTemps(); err != nil {
		return Rotated{}, err
	}
	j, err := journal.Open(r.Config.JournalPath())
	if err != nil {
		return Rotated{}, err
	}
	if n := len(j.Entries()); n > 0 {
		return Rotated{}, fmt.Errorf("%w (%d attempts)", ErrJournalBusy, n)
	}
	next, finished, err := r.prepare(ctx, current)
	if err != nil || finished {
		return Rotated{Identity: next, Resumed: finished, Finished: finished}, err
	}
	if err := r.confirm(ctx, current, next); err != nil {
		return Rotated{}, err
	}
	return Rotated{Identity: next}, nil
}

func (r Rotation) prepare(ctx context.Context, current *identity.Identity) (*identity.Identity, bool, error) {
	pending, err := loadPending(r.Store, current)
	if err != nil {
		return nil, false, err
	}
	if pending != nil && !current.SameKeys(pending) {
		next, done, err := r.settle(ctx, current, pending)
		if err != nil || done {
			return next, done, err
		}
	}
	keys, err := r.Generate()
	if err != nil {
		return nil, false, err
	}
	next, err := current.Next(keys)
	if err != nil {
		return nil, false, err
	}
	if err := r.SavePending(r.Store.PendingPath(), next); err != nil {
		return nil, false, fmt.Errorf("save the new keys before asking the server to take them: %w", err)
	}
	return next, false, nil
}

func (r Rotation) settle(ctx context.Context, current, pending *identity.Identity) (*identity.Identity, bool, error) {
	next, err := current.Next(pending.Keys)
	if err != nil {
		return nil, false, err
	}
	v, c, err := checkPending(ctx, r.Config, r.Log, current, next)
	if err != nil {
		return nil, false, fmt.Errorf("check whether the server took the new keys of the unfinished rotation: %w", err)
	}
	if c != nil {
		defer c.CloseIdle()
	}
	switch v {
	case keepCurrent:
		r.Log.Info("the server never took the keys of the unfinished rotation; they are discarded and fresh keys are made")
		return nil, false, r.Store.Discard()
	case takePending:
		if err := r.Promote(r.Store.PendingPath(), r.Store.Path(), next); err != nil {
			return nil, false, errors.Join(ErrNotSwitched, err)
		}
		return next, true, nil
	default:
		return nil, false, ErrNoKeyAccepted
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
	}
	if err := r.Promote(r.Store.PendingPath(), r.Store.Path(), next); err != nil {
		return errors.Join(ErrNotSwitched, err)
	}
	return nil
}
