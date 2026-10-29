package config

import settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"

const (
	EnvSMTPHost         = settingspec.EnvSMTPHost
	EnvSMTPPort         = settingspec.EnvSMTPPort
	EnvSMTPUsername     = settingspec.EnvSMTPUsername
	EnvSMTPPassword     = settingspec.EnvSMTPPassword
	EnvSMTPPasswordFile = "DHOW_SMTP_PASSWORD_FILE" //nolint:gosec // The name of an environment variable, not a credential.
	EnvSMTPFrom         = settingspec.EnvSMTPFrom
	EnvSMTPTLS          = settingspec.EnvSMTPTLS
	EnvOrgCreation      = settingspec.EnvOrgCreation

	CredentialSMTPPassword = "dhow-smtp-password" //nolint:gosec // The name of a systemd credential, not a credential.

	SMTPTLSStartTLS = settingspec.SMTPTLSStartTLS
	SMTPTLSImplicit = settingspec.SMTPTLSImplicit
	SMTPTLSNone     = settingspec.SMTPTLSNone

	OrgCreationInstanceOwner = settingspec.OrgCreationInstanceOwner
	OrgCreationAnyUser       = settingspec.OrgCreationAnyUser
)

type SMTP = settingspec.SMTP
