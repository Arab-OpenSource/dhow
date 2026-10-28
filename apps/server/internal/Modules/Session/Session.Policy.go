package session

import (
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
