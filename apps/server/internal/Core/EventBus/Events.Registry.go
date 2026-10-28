package eventbus

import "github.com/google/uuid"

const (
	SystemInstanceSetupCompleted = "system.instance.setupCompleted"

	IdentityOrganizationUpdated = "identity.organization.updated"

	AuthLogin                 = "auth.login"
	AuthLoginFailed           = "auth.login_failed"
	AuthLogout                = "auth.logout"
	AuthPasswordChanged       = "auth.password_changed"
	AuthSessionRevoked        = "auth.session_revoked"
	AuthSessionsRevokedOthers = "auth.sessions_revoked_others"

	AuthMfaPending                = "auth.mfa_pending"
	AuthMfaFailed                 = "auth.mfa_failed"
	AuthRecoveryCodeUsed          = "auth.recovery_code_used"
	AuthTotpEnabled               = "auth.totp_enabled"
	AuthTotpDisabled              = "auth.totp_disabled"
	AuthRecoveryCodesRegenerated  = "auth.recovery_codes_regenerated"
	AuthPasskeyAdded              = "auth.passkey_added"                //nolint:gosec // An audit action name, not a credential.
	AuthPasskeyRenamed            = "auth.passkey_renamed"              //nolint:gosec // An audit action name, not a credential.
	AuthPasskeyRemoved            = "auth.passkey_removed"              //nolint:gosec // An audit action name, not a credential.
	AuthPasskeySignCountRegressed = "auth.passkey_sign_count_regressed" //nolint:gosec // An audit action name, not a credential.

	RunnerRegistered               = "runner.registered"
	RunnerUpdated                  = "runner.updated"
	RunnerRevoked                  = "runner.revoked"
	RunnerDisabledCloneSuspected   = "runner.disabled_clone_suspected"
	RunnerKeysRotated              = "runner.keys_rotated"
	RunnerRegistrationTokenCreated = "runner.registration_token_created"
	RunnerRegistrationTokenRevoked = "runner.registration_token_revoked"

	JobCreated         = "job.created"
	JobCancelRequested = "job.cancel_requested"
)

func AuditedEvents() []string {
	return []string{
		SystemInstanceSetupCompleted,
		IdentityOrganizationUpdated,
		AuthLogin,
		AuthLoginFailed,
		AuthLogout,
		AuthPasswordChanged,
		AuthSessionRevoked,
		AuthSessionsRevokedOthers,
		AuthMfaPending,
		AuthMfaFailed,
		AuthRecoveryCodeUsed,
		AuthTotpEnabled,
		AuthTotpDisabled,
		AuthRecoveryCodesRegenerated,
		AuthPasskeyAdded,
		AuthPasskeyRenamed,
		AuthPasskeyRemoved,
		AuthPasskeySignCountRegressed,
		RunnerRegistered,
		RunnerUpdated,
		RunnerRevoked,
		RunnerDisabledCloneSuspected,
		RunnerKeysRotated,
		RunnerRegistrationTokenCreated,
		RunnerRegistrationTokenRevoked,
		JobCreated,
		JobCancelRequested,
	}
}

type Audited struct {
	OrganizationIDs []uuid.UUID
	ActorUserID     *uuid.UUID
	TargetType      string
	TargetID        string
	IP              string
	Data            map[string]string
}
