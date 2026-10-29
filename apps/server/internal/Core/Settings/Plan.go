package instancesettings

import (
	"slices"
	"sort"
	"strings"

	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
)

const keyLockedByPassword = "errors.settingLockedByPassword"

type planned struct {
	next        settingspec.Raw
	changed     []string
	clearSealed bool
}

type planState struct {
	snap    *Snapshot
	g       settingspec.Group
	base    settingspec.Raw
	next    settingspec.Raw
	locked  []apperrors.IErrorDetail
	invalid []apperrors.IErrorDetail
	clear   bool
	touched map[string]bool
}

func (p *planState) apply(c Change) error {
	k, ok := settingspec.KeyByField(p.g, c.Field)
	if !ok || p.touched[c.Field] {
		return apperrors.BadRequest()
	}
	p.touched[c.Field] = true
	if by := p.snap.LockedBy(k.Env); by != "" {
		switch {
		case k.Secret && c.Value == "":
			p.clear = p.clear || p.snap.Stored[p.g].Sealed || p.base.Has(k.Env)
			delete(p.next, k.Env)
		case by == k.Env:
			p.locked = append(p.locked, detail(k.Field, apperrors.KeySettingLockedByEnv))
		default:
			p.locked = append(p.locked, detail(k.Field, keyLockedByPassword))
		}
		return nil
	}
	if c.Value == "" {
		delete(p.next, k.Env)
		return nil
	}
	if e := settingspec.ValidateValue(k.Env, c.Value); e != nil {
		p.invalid = append(p.invalid, detail(k.Field, invalidPrefix+e.Rule))
		return nil
	}
	p.next[k.Env] = c.Value
	return nil
}

func (s *Store) plan(snap *Snapshot, g settingspec.Group, base settingspec.Raw, changes []Change) (planned, error) {
	p := &planState{snap: snap, g: g, base: base, next: settingspec.Raw{}, touched: map[string]bool{}}
	for _, k := range settingspec.KeysOf(g) {
		if v, ok := base[k.Env]; ok {
			p.next[k.Env] = v
		}
	}
	for _, c := range changes {
		if err := p.apply(c); err != nil {
			return planned{}, err
		}
	}
	if len(p.locked) > 0 {
		return planned{}, apperrors.SettingLockedByEnv(p.locked...)
	}
	merged := snap.merged()
	for _, k := range settingspec.KeysOf(g) {
		if snap.Locked(k.Env) {
			continue
		}
		delete(merged, k.Env)
		if v, ok := p.next[k.Env]; ok {
			merged[k.Env] = v
		}
	}
	if g == settingspec.GroupMail {
		p.mailRules(merged)
	}
	if len(p.invalid) == 0 {
		if _, errs := settingspec.Parse(merged); len(errs) > 0 {
			for _, e := range errs {
				if k, ok := settingspec.KeyByEnv(e.Env); ok && k.Group == g {
					p.invalid = append(p.invalid, detail(k.Field, invalidPrefix+e.Rule))
				}
			}
		}
	}
	if len(p.invalid) > 0 {
		return planned{}, apperrors.SettingInvalid(p.invalid...)
	}
	changed := changedFields(g, base, p.next)
	if p.clear {
		changed = appendSecretFields(g, changed)
	}
	return planned{next: p.next, changed: changed, clearSealed: p.clear}, nil
}

func (p *planState) mailRules(merged settingspec.Raw) {
	host := strings.TrimSpace(merged.Get(settingspec.EnvSMTPHost))
	if p.base.Get(settingspec.EnvSMTPPassword) != "" && host != strings.TrimSpace(p.base.Get(settingspec.EnvSMTPHost)) &&
		!p.touched["Password"] && !p.snap.Locked(settingspec.EnvSMTPHost) {
		p.invalid = append(p.invalid, detail("Password", invalidPrefix+settingspec.RulePasswordAgain))
	}
	tls := strings.ToLower(strings.TrimSpace(merged.Get(settingspec.EnvSMTPTLS)))
	fromEnv := p.snap.Locked(settingspec.EnvSMTPTLS) && p.snap.Locked(settingspec.EnvSMTPHost)
	if tls == settingspec.SMTPTLSNone && host != "" && !fromEnv && !settingspec.LoopbackHost(host) {
		p.invalid = append(p.invalid, detail("Tls", invalidPrefix+settingspec.RuleSMTPPlainRemote))
	}
}

func appendSecretFields(g settingspec.Group, changed []string) []string {
	for _, k := range settingspec.KeysOf(g) {
		if k.Secret && !slices.Contains(changed, k.Field) {
			changed = append(changed, k.Field)
		}
	}
	sort.Strings(changed)
	return changed
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
