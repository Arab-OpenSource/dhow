package instancesettings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Khaymah/dhow/apps/server/internal/Database/Tx"
)

const invalidPrefix = "settings.invalid."

type Change struct {
	Field string
	Value string
}

type Result struct {
	Snapshot *Snapshot
	Changed  []string
	Version  int64
}

func detail(field, message string) apperrors.IErrorDetail {
	return apperrors.IErrorDetail{Field: field, Message: message}
}

func (s *Store) Check(g settingspec.Group, version int64, changes []Change) error {
	snap := s.Current()
	if _, _, err := s.plan(snap, g, snap.Stored[g].Raw, changes); err != nil {
		return err
	}
	if snap.Version(g) > version {
		return apperrors.SettingChanged()
	}
	return nil
}

func (s *Store) plan(snap *Snapshot, g settingspec.Group, base settingspec.Raw, changes []Change) (settingspec.Raw, []string, error) {
	next := settingspec.Raw{}
	for _, k := range settingspec.KeysOf(g) {
		if v, ok := base[k.Env]; ok {
			next[k.Env] = v
		}
	}
	var locked, invalid []apperrors.IErrorDetail
	seen := map[string]bool{}
	for _, c := range changes {
		k, ok := settingspec.KeyByField(g, c.Field)
		if !ok || seen[c.Field] {
			return nil, nil, apperrors.BadRequest()
		}
		seen[c.Field] = true
		if snap.Locked(k.Env) {
			locked = append(locked, detail(k.Field, apperrors.KeySettingLockedByEnv))
			continue
		}
		if c.Value == "" {
			delete(next, k.Env)
			continue
		}
		if e := settingspec.ValidateValue(k.Env, c.Value); e != nil {
			invalid = append(invalid, detail(k.Field, invalidPrefix+e.Rule))
			continue
		}
		next[k.Env] = c.Value
	}
	if len(locked) > 0 {
		return nil, nil, apperrors.SettingLockedByEnv(locked...)
	}
	if len(invalid) > 0 {
		return nil, nil, apperrors.SettingInvalid(invalid...)
	}
	merged := snap.merged()
	for _, k := range settingspec.KeysOf(g) {
		if snap.Locked(k.Env) {
			continue
		}
		delete(merged, k.Env)
		if v, ok := next[k.Env]; ok {
			merged[k.Env] = v
		}
	}
	if _, errs := settingspec.Parse(merged); len(errs) > 0 {
		for _, e := range errs {
			if k, ok := settingspec.KeyByEnv(e.Env); ok && k.Group == g {
				invalid = append(invalid, detail(k.Field, invalidPrefix+e.Rule))
			}
		}
		if len(invalid) > 0 {
			return nil, nil, apperrors.SettingInvalid(invalid...)
		}
	}
	return next, changedFields(g, base, next), nil
}

func changedFields(g settingspec.Group, before, after settingspec.Raw) []string {
	var out []string
	for _, k := range settingspec.KeysOf(g) {
		b, bok := before[k.Env]
		a, aok := after[k.Env]
		if bok != aok || b != a {
			out = append(out, k.Field)
		}
	}
	sort.Strings(out)
	return out
}

func (s *Store) Update(ctx context.Context, g settingspec.Group, version int64, changes []Change, actor uuid.UUID) (Result, error) {
	if s.db == nil {
		return Result{}, apperrors.ServiceUnavailable()
	}
	var res Result
	err := dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		row, err := q.LockInstanceSetting(ctx, string(g))
		exists := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		base := settingspec.Raw{}
		var current int64
		if exists {
			current = row.Version
			if base, err = s.decode(g, row.Value, row.Secrets); err != nil {
				return err
			}
		}
		if current != version {
			return apperrors.SettingChanged()
		}
		next, changed, err := s.plan(s.Current(), g, base, changes)
		if err != nil {
			return err
		}
		res.Changed, res.Version = changed, current
		if len(changed) == 0 {
			return nil
		}
		value, secrets, err := s.encode(g, next)
		if err != nil {
			return err
		}
		if exists {
			res.Version, err = q.UpdateInstanceSetting(ctx, sqlc.UpdateInstanceSettingParams{
				Value: value, Secrets: secrets, UpdatedBy: &actor, ID: row.Id, Version: version,
			})
		} else {
			res.Version, err = q.InsertInstanceSetting(ctx, sqlc.InsertInstanceSettingParams{
				GroupName: string(g), Value: value, Secrets: secrets, UpdatedBy: &actor,
			})
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.SettingChanged()
		}
		if err != nil {
			return err
		}
		return q.NotifySettingsChanged(ctx, string(g))
	})
	if err != nil {
		return Result{}, err
	}
	if len(res.Changed) > 0 {
		if err := s.Load(ctx); err != nil {
			return Result{}, err
		}
	}
	res.Snapshot = s.Current()
	return res, nil
}

func (s *Store) encode(g settingspec.Group, raw settingspec.Raw) ([]byte, []byte, error) {
	values := map[string]any{}
	secrets := map[string]string{}
	for _, k := range settingspec.KeysOf(g) {
		v, ok := raw[k.Env]
		if !ok {
			continue
		}
		if k.Secret {
			secrets[k.Field] = v
			continue
		}
		typed, err := jsonOf(k, v)
		if err != nil {
			return nil, nil, fmt.Errorf("encode %s: %w", k.Env, err)
		}
		values[k.Field] = typed
	}
	value, err := json.Marshal(values)
	if err != nil {
		return nil, nil, fmt.Errorf("encode instance settings: %w", err)
	}
	if len(secrets) == 0 {
		return value, nil, nil
	}
	plain, err := json.Marshal(secrets)
	if err != nil {
		return nil, nil, fmt.Errorf("encode instance setting secrets: %w", err)
	}
	defer clear(plain)
	sealed, err := s.sealer.Seal(plain, sealContext(g))
	if err != nil {
		return nil, nil, err
	}
	return value, sealed, nil
}
