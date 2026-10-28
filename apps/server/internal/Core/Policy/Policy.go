package policy

import "github.com/google/uuid"

type Role string

const (
	RoleOwner      Role = "owner"
	RoleAdmin      Role = "admin"
	RoleMaintainer Role = "maintainer"
	RoleDeveloper  Role = "developer"
	RoleViewer     Role = "viewer"
)

func Roles() []Role {
	return []Role{RoleOwner, RoleAdmin, RoleMaintainer, RoleDeveloper, RoleViewer}
}

func ParseRole(v string) (Role, bool) {
	for _, r := range Roles() {
		if string(r) == v {
			return r, true
		}
	}
	return "", false
}

type Action string

const (
	OrgRead           Action = "org.read"
	OrgUpdate         Action = "org.update"
	OrgLeave          Action = "org.leave"
	OrgTransfer       Action = "org.transfer"
	MemberRead        Action = "member.read"
	MemberManage      Action = "member.manage"
	AuditRead         Action = "audit.read"
	SessionManageSelf Action = "session.manage_self"
	OrgCreate         Action = "org.create"
)

func Actions() []Action {
	return []Action{OrgRead, OrgUpdate, OrgLeave, OrgTransfer, MemberRead, MemberManage, AuditRead, SessionManageSelf, OrgCreate}
}

func InstanceActions() []Action { return []Action{OrgCreate} }

func IsInstanceAction(a Action) bool {
	for _, ia := range InstanceActions() {
		if ia == a {
			return true
		}
	}
	return false
}

func Grants(role Role) []Action {
	switch role {
	case RoleOwner:
		return []Action{OrgRead, OrgUpdate, OrgLeave, OrgTransfer, MemberRead, MemberManage, AuditRead, SessionManageSelf}
	case RoleAdmin:
		return []Action{OrgRead, OrgUpdate, OrgLeave, MemberRead, MemberManage, AuditRead, SessionManageSelf}
	case RoleMaintainer, RoleDeveloper:
		return []Action{OrgRead, OrgLeave, MemberRead, SessionManageSelf}
	case RoleViewer:
		return []Action{OrgRead, OrgLeave, SessionManageSelf}
	default:
		return nil
	}
}

func Allows(role Role, action Action) bool {
	for _, a := range Grants(role) {
		if a == action {
			return true
		}
	}
	return false
}

type Membership struct {
	OrganizationID uuid.UUID
	Role           Role
}

type Subject struct {
	UserID        uuid.UUID
	SessionID     uuid.UUID
	InstanceOwner bool
	Memberships   []Membership
}

func (s Subject) RoleIn(organizationID uuid.UUID) (Role, bool) {
	for _, m := range s.Memberships {
		if m.OrganizationID == organizationID {
			return m.Role, true
		}
	}
	return "", false
}

type Resource struct {
	OrganizationID uuid.UUID
}

func Can(s Subject, action Action, r Resource) bool {
	if s.UserID == uuid.Nil || IsInstanceAction(action) {
		return false
	}
	if action == SessionManageSelf {
		return r.OrganizationID == uuid.Nil
	}
	if r.OrganizationID == uuid.Nil {
		return false
	}
	role, ok := s.RoleIn(r.OrganizationID)
	return ok && Allows(role, action)
}

type InstanceSettings struct {
	AnyUserCreatesOrganizations bool
}

func CanInstance(s Subject, action Action, inst InstanceSettings) bool {
	if s.UserID == uuid.Nil || !IsInstanceAction(action) {
		return false
	}
	switch action {
	case OrgCreate:
		return s.InstanceOwner || inst.AnyUserCreatesOrganizations
	default:
		return false
	}
}
