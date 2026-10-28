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
	MemberRead        Action = "member.read"
	MemberManage      Action = "member.manage"
	AuditRead         Action = "audit.read"
	SessionManageSelf Action = "session.manage_self"
	RunnerRead        Action = "runner.read"
	RunnerManage      Action = "runner.manage"
)

func Actions() []Action {
	return []Action{OrgRead, OrgUpdate, MemberRead, MemberManage, AuditRead, SessionManageSelf, RunnerRead, RunnerManage}
}

func Grants(role Role) []Action {
	switch role {
	case RoleOwner, RoleAdmin:
		return []Action{OrgRead, OrgUpdate, MemberRead, MemberManage, AuditRead, SessionManageSelf, RunnerRead, RunnerManage}
	case RoleMaintainer, RoleDeveloper:
		return []Action{OrgRead, MemberRead, SessionManageSelf, RunnerRead}
	case RoleViewer:
		return []Action{OrgRead, SessionManageSelf}
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
	UserID      uuid.UUID
	SessionID   uuid.UUID
	Memberships []Membership
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
	if s.UserID == uuid.Nil {
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
