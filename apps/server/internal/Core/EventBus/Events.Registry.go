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
