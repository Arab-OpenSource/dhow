package settings

import (
	"strconv"
	"strings"

	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
	instancesettings "github.com/Khaymah/dhow/apps/server/internal/Core/Settings"
	gen "github.com/Khaymah/dhow/apps/server/internal/Modules/Settings/Gen"
)

type view struct {
	snap *instancesettings.Snapshot
}

func (v view) meta(env string) (gen.SettingSource, bool, *string) {
	by := v.snap.LockedBy(env)
	if by == "" {
		return gen.SettingSource(v.snap.Sources[env]), false, nil
	}
	return gen.SettingSource(v.snap.Sources[env]), true, &by
}

func (v view) text(env, value string) gen.StringSetting {
	src, locked, by := v.meta(env)
	return gen.StringSetting{Value: value, Source: src, Locked: locked, LockedBy: by, Env: env}
}

func (v view) number(env string, value int64) gen.IntegerSetting {
	src, locked, by := v.meta(env)
	return gen.IntegerSetting{Value: value, Source: src, Locked: locked, LockedBy: by, Env: env}
}

func (v view) flag(env string, value bool) gen.BooleanSetting {
	src, locked, by := v.meta(env)
	return gen.BooleanSetting{Value: value, Source: src, Locked: locked, LockedBy: by, Env: env}
}

func (v view) secret(env string) gen.SecretSetting {
	return gen.SecretSetting{Set: v.snap.SecretSet(env), Locked: v.snap.Locked(env), Saved: v.snap.SecretStored(env), Env: env}
}

func settingsToGen(snap *instancesettings.Snapshot) gen.InstanceSettings {
	v := view{snap: snap}
	values := snap.Values
	return gen.InstanceSettings{
		Mail: gen.MailSettings{
			Version:  snap.Version(settingspec.GroupMail),
			Host:     v.text(settingspec.EnvSMTPHost, values.Mail.Host),
			Port:     v.number(settingspec.EnvSMTPPort, int64(values.Mail.Port)),
			Tls:      v.text(settingspec.EnvSMTPTLS, values.Mail.TLS),
			Username: v.text(settingspec.EnvSMTPUsername, values.Mail.Username),
			Password: v.secret(settingspec.EnvSMTPPassword),
			From:     v.text(settingspec.EnvSMTPFrom, strings.TrimSpace(snap.Text(settingspec.EnvSMTPFrom))),
		},
		Organizations: gen.OrganizationSettings{
			Version:  snap.Version(settingspec.GroupOrganizations),
			Creation: v.text(settingspec.EnvOrgCreation, values.OrgCreation),
		},
		Passwords: gen.PasswordSettings{
			Version:       snap.Version(settingspec.GroupPasswords),
			BreachedCheck: v.flag(settingspec.EnvBreachedPasswordCheck, values.BreachedPasswordCheck),
		},
		RateLimits: gen.RateLimitSettings{
			Version:                snap.Version(settingspec.GroupRateLimits),
			Public:                 v.number(settingspec.EnvRateLimitPublic, int64(values.RateLimits.Public)),
			InvitesPerUser:         v.number(settingspec.EnvRateLimitInvitesUser, int64(values.RateLimits.InvitesUser)),
			InvitesPerOrganization: v.number(settingspec.EnvRateLimitInvitesOrg, int64(values.RateLimits.InvitesOrg)),
		},
		Logs: gen.LogSettings{
			Version:        snap.Version(settingspec.GroupLogs),
			RetentionDays:  v.number(settingspec.EnvLogRetentionDays, int64(values.Logs.RetentionDays)),
			JobLogMaxBytes: v.number(settingspec.EnvJobLogMaxBytes, values.Logs.JobLogMaxBytes),
		},
	}
}

type changes []instancesettings.Change

func (c *changes) text(field string, v *string) {
	if v != nil {
		*c = append(*c, instancesettings.Change{Field: field, Value: *v})
	}
}

func (c *changes) number(field string, v *int64) {
	if v != nil {
		*c = append(*c, instancesettings.Change{Field: field, Value: strconv.FormatInt(*v, 10)})
	}
}

func (c *changes) small(field string, v *int) {
	if v != nil {
		*c = append(*c, instancesettings.Change{Field: field, Value: strconv.Itoa(*v)})
	}
}

func (c *changes) flag(field string, v *bool) {
	if v != nil {
		*c = append(*c, instancesettings.Change{Field: field, Value: strconv.FormatBool(*v)})
	}
}

func mailChanges(b *gen.UpdateMailSettingsRequest) changes {
	var out changes
	out.text("Host", b.Host)
	out.small("Port", b.Port)
	if b.Tls != nil {
		tls := string(*b.Tls)
		out.text("Tls", &tls)
	}
	out.text("Username", b.Username)
	out.text("Password", b.Password)
	out.text("From", b.From)
	return out
}

func organizationChanges(b *gen.UpdateOrganizationSettingsRequest) changes {
	var out changes
	if b.Creation != nil {
		v := string(*b.Creation)
		out.text("Creation", &v)
	}
	return out
}

func passwordChanges(b *gen.UpdatePasswordSettingsRequest) changes {
	var out changes
	out.flag("BreachedCheck", b.BreachedCheck)
	return out
}

func rateLimitChanges(b *gen.UpdateRateLimitSettingsRequest) changes {
	var out changes
	out.small("Public", b.Public)
	out.small("InvitesPerUser", b.InvitesPerUser)
	out.small("InvitesPerOrganization", b.InvitesPerOrganization)
	return out
}

func logChanges(b *gen.UpdateLogSettingsRequest) changes {
	var out changes
	out.small("RetentionDays", b.RetentionDays)
	out.number("JobLogMaxBytes", b.JobLogMaxBytes)
	return out
}
