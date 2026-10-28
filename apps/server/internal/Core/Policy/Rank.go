package policy

func Rank(r Role) int {
	switch r {
	case RoleOwner:
		return 5
	case RoleAdmin:
		return 4
	case RoleMaintainer:
		return 3
	case RoleDeveloper:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

func AtLeast(r, floor Role) bool { return Rank(r) > 0 && Rank(r) >= Rank(floor) }

func Effective(direct Role, bindings ...Role) Role {
	best := direct
	if Rank(best) == 0 {
		return ""
	}
	for _, b := range bindings {
		if b == RoleOwner {
			continue
		}
		if Rank(b) > Rank(best) {
			best = b
		}
	}
	return best
}

func EffectiveOf(direct string, bindings []string) (Role, bool) {
	role, ok := ParseRole(direct)
	if !ok {
		return "", false
	}
	extra := make([]Role, 0, len(bindings))
	for _, b := range bindings {
		if r, ok := ParseRole(b); ok {
			extra = append(extra, r)
		}
	}
	return Effective(role, extra...), true
}

func Bindable(r Role) bool { return r != RoleOwner && Rank(r) > 0 }

func CanGrant(actor, role Role) bool { return Rank(role) > 0 && Rank(actor) >= Rank(role) }

func CanResetMfa(actor, target Role, targetInstanceOwner bool) bool {
	if targetInstanceOwner {
		target = RoleOwner
	}
	return Allows(actor, MemberManage) && Rank(actor) > Rank(target)
}

func CanManage(actor, target Role) bool {
	if target == RoleOwner {
		return actor == RoleOwner
	}
	return Allows(actor, MemberManage) && Rank(actor) >= Rank(target)
}
