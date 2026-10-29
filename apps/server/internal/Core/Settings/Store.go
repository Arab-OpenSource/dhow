package instancesettings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
	appcrypto "github.com/Khaymah/dhow/apps/server/internal/Core/Crypto"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Khaymah/dhow/apps/server/internal/Database/Tx"
)

type Source string

const (
	SourceEnv      Source = "env"
	SourceDatabase Source = "database"
	SourceDefault  Source = "default"

	ChangedChannel = "dhow_settings_changed"
	contextPrefix  = "instance-settings:"
)

var ErrUndecryptable = errors.New("cannot be decrypted with the current DHOW_SECRET_KEY")

type DB interface {
	sqlc.DBTX
	dbtx.Beginner
}

type Listener func(ctx context.Context) (*pgx.Conn, error)

type Stored struct {
	Raw       settingspec.Raw
	Version   int64
	UpdatedAt *time.Time
	UpdatedBy *uuid.UUID
}

type Snapshot struct {
	Values  settingspec.Settings
	Sources map[string]Source
	Stored  map[settingspec.Group]Stored
	env     settingspec.Raw
}

func (s *Snapshot) Locked(env string) bool { return s.env.Has(env) }

func (s *Snapshot) SecretSet(env string) bool {
	if s.env.Get(env) != "" {
		return true
	}
	k, ok := settingspec.KeyByEnv(env)
	return ok && s.Stored[k.Group].Raw.Get(env) != ""
}

func (s *Snapshot) Version(g settingspec.Group) int64 { return s.Stored[g].Version }

func (s *Snapshot) Text(env string) string {
	k, ok := settingspec.KeyByEnv(env)
	if !ok || k.Secret {
		return ""
	}
	if v, ok := s.env[env]; ok {
		return v
	}
	return s.Stored[k.Group].Raw.Get(env)
}

func (s *Snapshot) merged() settingspec.Raw {
	out := settingspec.Raw{}
	for _, st := range s.Stored {
		for k, v := range st.Raw {
			out[k] = v
		}
	}
	for k, v := range s.env {
		out[k] = v
	}
	return out
}

func (s *Snapshot) Instance() policy.InstanceSettings {
	return policy.InstanceSettings{AnyUserCreatesOrganizations: s.Values.AnyUserCreatesOrganizations()}
}

type Options struct {
	DB        DB
	SecretKey []byte
	Env       settingspec.Raw
	Listen    Listener
	Log       *slog.Logger
	Now       func() time.Time
}

type Store struct {
	db      DB
	sealer  *appcrypto.Sealer
	env     settingspec.Raw
	listen  Listener
	log     *slog.Logger
	now     func() time.Time
	mu      sync.Mutex
	current atomic.Pointer[Snapshot]
}

func New(o Options) (*Store, error) {
	sealer, err := appcrypto.NewSealer(o.SecretKey, config.InstanceSettingsSealLabel)
	if err != nil {
		return nil, err
	}
	now := o.Now
	if now == nil {
		now = time.Now
	}
	env := o.Env.Clone()
	s := &Store{db: o.DB, sealer: sealer, env: env, listen: o.Listen, log: o.Log, now: now}
	snap, err := s.build(map[settingspec.Group]Stored{})
	if err != nil && s.db == nil {
		return nil, err
	}
	if err != nil {
		snap = &Snapshot{Values: settingspec.Defaults(), Sources: map[string]Source{}, Stored: map[settingspec.Group]Stored{}, env: env}
	}
	s.current.Store(snap)
	return s, nil
}

func (s *Store) Current() *Snapshot { return s.current.Load() }

func (s *Store) Instance() policy.InstanceSettings { return s.Current().Instance() }

func (s *Store) Load(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, err := s.read(ctx, sqlc.New(s.db))
	if err != nil {
		return err
	}
	snap, err := s.build(stored)
	if err != nil {
		return err
	}
	s.current.Store(snap)
	return nil
}

func (s *Store) Reload(ctx context.Context) {
	if err := s.Load(ctx); err != nil {
		s.log.ErrorContext(ctx, "instance settings were not reloaded; the previous values stay in effect", "error", err)
	}
}

func (s *Store) read(ctx context.Context, q *sqlc.Queries) (map[settingspec.Group]Stored, error) {
	rows, err := q.ListInstanceSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("read instance settings: %w", err)
	}
	out := make(map[settingspec.Group]Stored, len(rows))
	var errs []error
	for _, r := range rows {
		g, ok := settingspec.ParseGroup(r.Group)
		if !ok {
			errs = append(errs, fmt.Errorf("instance setting group %q in the database is unknown", r.Group))
			continue
		}
		raw, err := s.decode(g, r.Value, r.Secrets)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		at := r.UpdatedAt
		out[g] = Stored{Raw: raw, Version: r.Version, UpdatedAt: &at, UpdatedBy: r.UpdatedBy}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return out, nil
}

func (s *Store) build(stored map[settingspec.Group]Stored) (*Snapshot, error) {
	snap := &Snapshot{Stored: stored, env: s.env, Sources: make(map[string]Source, len(settingspec.Keys()))}
	for _, k := range settingspec.Keys() {
		switch {
		case s.env.Has(k.Env):
			snap.Sources[k.Env] = SourceEnv
		case stored[k.Group].Raw.Has(k.Env):
			snap.Sources[k.Env] = SourceDatabase
		default:
			snap.Sources[k.Env] = SourceDefault
		}
	}
	values, errs := settingspec.Parse(snap.merged())
	if len(errs) > 0 {
		out := make([]error, 0, len(errs))
		for _, e := range errs {
			out = append(out, fmt.Errorf("instance setting %s (from the %s): %s", e.Env, describe(snap.Sources[e.Env]), e.Message))
		}
		return nil, errors.Join(out...)
	}
	snap.Values = values
	return snap, nil
}

func describe(src Source) string {
	switch src {
	case SourceEnv:
		return "environment"
	case SourceDatabase:
		return "database"
	default:
		return "default"
	}
}

func sealContext(g settingspec.Group) []byte { return []byte(contextPrefix + string(g)) }

func (s *Store) decode(g settingspec.Group, value, secrets []byte) (settingspec.Raw, error) {
	raw := settingspec.Raw{}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(value, &fields); err != nil {
		return nil, fmt.Errorf("instance setting group %s in the database is not a JSON object", g)
	}
	for name, v := range fields {
		k, ok := settingspec.KeyByField(g, name)
		if !ok || k.Secret {
			return nil, fmt.Errorf("instance setting %s.%s in the database is unknown", g, name)
		}
		text, err := textOf(k, v)
		if err != nil {
			return nil, fmt.Errorf("instance setting %s (from the database): %w", k.Env, err)
		}
		if e := settingspec.ValidateValue(k.Env, text); e != nil {
			return nil, fmt.Errorf("instance setting %s (from the database): %s", k.Env, e.Message)
		}
		raw[k.Env] = text
	}
	if len(secrets) == 0 {
		return raw, nil
	}
	plain, err := s.sealer.Open(secrets, sealContext(g))
	if err != nil {
		return nil, fmt.Errorf("instance setting secrets of %s (from the database): %w", g, ErrUndecryptable)
	}
	defer clear(plain)
	var sealed map[string]string
	if err := json.Unmarshal(plain, &sealed); err != nil {
		return nil, fmt.Errorf("instance setting secrets of %s (from the database) are malformed", g)
	}
	for name, v := range sealed {
		k, ok := settingspec.KeyByField(g, name)
		if !ok || !k.Secret {
			return nil, fmt.Errorf("instance setting secret %s.%s in the database is unknown", g, name)
		}
		if e := settingspec.ValidateValue(k.Env, v); e != nil {
			return nil, fmt.Errorf("instance setting %s (from the database): %s", k.Env, e.Message)
		}
		raw[k.Env] = v
	}
	return raw, nil
}
