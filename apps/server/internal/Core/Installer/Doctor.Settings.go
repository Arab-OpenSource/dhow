package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
)

type SettingsReader interface {
	StoredSettings(ctx context.Context, url string) (map[string]bool, error)
}

func (PostgresChecker) StoredSettings(ctx context.Context, url string) (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(ctx, databaseCheckTimeout)
	defer cancel()
	pool, err := config.OpenDatabase(ctx, url)
	if err != nil {
		return nil, err
	}
	defer pool.Close()
	rows, err := sqlc.New(pool).ListInstanceSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("read instance settings: %w", err)
	}
	out := map[string]bool{}
	for _, r := range rows {
		g, ok := settingspec.ParseGroup(r.Group)
		if !ok {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(r.Value, &fields); err != nil {
			return nil, fmt.Errorf("instance settings of %s are not a JSON object", g)
		}
		for _, k := range settingspec.KeysOf(g) {
			_, stored := fields[k.Field]
			if stored || (k.Secret && len(r.Secrets) > 0) {
				out[k.Env] = true
			}
		}
	}
	return out, nil
}

func (d Doctor) envSettings() map[string]bool {
	path := EnvFilePath
	if d.A.Mode == ModeCompose {
		path = ComposeEnvPath
	}
	entries := ParseEnv(d.b.readExisting(path))
	out := map[string]bool{}
	for _, k := range settingspec.Keys() {
		if strings.TrimSpace(EnvValue(entries, k.Env)) != "" {
			out[k.Env] = true
		}
	}
	if EnvValue(entries, config.EnvSMTPPasswordFile) != "" || exists(d.Sys, CredentialsDir+"/"+config.CredentialSMTPPassword) {
		out[config.EnvSMTPPassword] = true
	}
	return out
}

func (d Doctor) storedSettings(ctx context.Context) (map[string]bool, bool) {
	if d.Settings == nil || d.A.Mode == ModeCompose {
		return nil, false
	}
	url, err := d.secretContent(DatabaseURLPath)
	if err != nil {
		return nil, false
	}
	stored, err := d.Settings.StoredSettings(ctx, url)
	if err != nil {
		return nil, false
	}
	return stored, true
}

func (d Doctor) settingSources(ctx context.Context) []Finding {
	env := d.envSettings()
	stored, known := d.storedSettings(ctx)
	out := make([]Finding, 0, len(settingspec.Keys()))
	for _, k := range settingspec.Keys() {
		var source Key
		switch {
		case env[k.Env]:
			source = KeySourceEnv
		case !known:
			source = KeySourceUnknown
		case stored[k.Env]:
			source = KeySourceDatabase
		default:
			source = KeySourceDefault
		}
		out = append(out, Finding{Severity: SeverityInfo, Title: d.M.T(KeyCheckSetting, k.Env, d.M.T(source))})
	}
	return out
}
