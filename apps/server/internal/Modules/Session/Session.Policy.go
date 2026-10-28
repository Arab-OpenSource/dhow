package session

import (
	"time"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
)

type resolvedMembership struct {
	row sqlc.ListUserMembershipsRow
	policy.Membership
}

func membershipsOf(rows []sqlc.ListUserMembershipsRow, hasFactor bool) []resolvedMembership {
	out := make([]resolvedMembership, 0, len(rows))
	for _, r := range rows {
		role, ok := policy.EffectiveOf(r.Role, r.TeamRoles)
		if !ok {
			continue
		}
		rule, known := policy.ParseSecondFactorRule(r.RequireSecondFactor)
		if !known {
			rule = policy.SecondFactorEveryone
		}
		due := policy.SecondFactorDue(policy.SecondFactorState{
			Rule: rule, Since: r.RequireSecondFactorSince, JoinedAt: r.JoinedAt, Role: role, HasFactor: hasFactor,
		}, config.SecondFactorGrace)
		out = append(out, resolvedMembership{row: r, Membership: policy.Membership{OrganizationID: r.OrganizationId, Role: role, SecondFactorDue: due}})
	}
	return out
}

func policyMemberships(ms []resolvedMembership) []policy.Membership {
	out := make([]policy.Membership, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Membership)
	}
	return out
}

func privileged(instanceOwner bool, ms []policy.Membership) bool {
	if instanceOwner {
		return true
	}
	for _, m := range ms {
		if policy.AtLeast(m.Role, policy.RoleAdmin) {
			return true
		}
	}
	return false
}

func idleTimeout(instanceOwner bool, ms []policy.Membership) time.Duration {
	if privileged(instanceOwner, ms) {
		return config.SessionPrivilegedIdleTimeout
	}
	return config.SessionIdleTimeout
}

func absoluteExpiry(created, stored time.Time) time.Time {
	if limit := created.Add(config.SessionAbsoluteTimeout); limit.Before(stored) {
		return limit
	}
	return stored
}

func idleExpired(lastSeen time.Time, idle time.Duration, now time.Time) bool {
	return !now.Before(lastSeen.Add(idle))
}
