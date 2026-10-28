import type { TRouteHandler } from '@/Core/Testing/Backend';
import { envelope, ORG_ID } from '@/Core/Testing/Fetch';

export const TEAM_ID = '0198a000-0000-7000-8000-0000000000d1';
export const BINDING_ID = '0198a000-0000-7000-8000-0000000000d2';
export const INVITATION_ID = '0198a000-0000-7000-8000-0000000000e1';
export const INVITE_LINK = 'https://dhow.example.com/invite#AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA';

export interface IOrgCall {
  method: string;
  path: string;
}

export interface IOrgBackend {
  calls: IOrgCall[];
  routes: TRouteHandler[];
}

const invitation = (email: string, role: string, status = 'disabled') => ({
  Id: INVITATION_ID,
  Email: email,
  Role: role,
  InvitedByName: 'Sabry Dawood',
  CreatedAt: '2026-10-27T08:00:00Z',
  ExpiresAt: '2026-11-03T08:00:00Z',
  SentAt: '2026-10-27T08:00:00Z',
  SendCount: 1,
  EmailStatus: status,
  Expired: false,
});

const team = (role: string | null, members: { UserId: string; Name: string }[]) => ({
  Id: TEAM_ID,
  Name: 'Backend',
  Description: 'Payments API',
  MemberCount: members.length,
  CreatedAt: '2026-10-20T08:00:00Z',
  Bindings: role === null ? [] : [{ Id: BINDING_ID, Scope: 'org', Role: role, CreatedAt: '2026-10-20T08:00:00Z' }],
  Members: members.map((member) => ({ ...member, Email: 'hana@future.dev', Role: 'developer', AddedAt: '2026-10-21T08:00:00Z' })),
});

export const orgBackend = (emailStatus = 'disabled'): IOrgBackend => {
  const calls: IOrgCall[] = [];
  const state = {
    teamRole: 'developer' as string | null,
    teamMembers: [{ UserId: '0198a000-0000-7000-8000-0000000000b3', Name: 'Hana Samir' }],
    invitations: [invitation('ali@future.dev', 'developer')],
  };
  const base = `/api/v1/organizations/${ORG_ID}`;
  const handler: TRouteHandler = (request, url) => {
    const path = url.pathname;
    if (!path.startsWith(base + '/') || path.endsWith('/members') || path.includes('/audit-events')) return undefined;
    const rest = path.slice(base.length);
    calls.push({ method: request.method, path: rest });
    if (rest === '/teams' && request.method === 'GET') {
      const detail: Partial<ReturnType<typeof team>> = team(state.teamRole, state.teamMembers);
      delete detail.Members;
      return envelope({ Success: true, Data: [detail] });
    }
    if (rest.startsWith(`/teams/${TEAM_ID}`)) {
      if (rest.endsWith('/role-bindings') || rest.includes('/role-bindings/')) state.teamRole = request.method === 'DELETE' ? null : 'admin';
      if (rest.endsWith('/members') && request.method === 'POST') state.teamMembers = [...state.teamMembers, { UserId: '0198a000-0000-7000-8000-0000000000b2', Name: 'Mona Adel' }];
      const detail = team(state.teamRole, state.teamMembers);
      if (rest === `/teams/${TEAM_ID}` && (request.method === 'GET' || request.method === 'PATCH')) return envelope({ Success: true, Data: detail });
      if (request.method === 'DELETE' && rest === `/teams/${TEAM_ID}`) return envelope({ Success: true, Data: { Revoked: 0 } });
      return envelope({ Success: true, Data: { Team: detail, Revoked: 1 } });
    }
    if (rest === '/teams' && request.method === 'POST') return envelope({ Success: true, Data: team(null, []) }, 201);
    if (rest === '/invitations' && request.method === 'GET') return envelope({ Success: true, Data: state.invitations });
    if (rest === '/invitations' && request.method === 'POST') {
      const created = invitation('new@future.dev', 'viewer', emailStatus);
      return envelope({ Success: true, Data: { Invitation: created, Link: INVITE_LINK } }, 201);
    }
    if (rest.startsWith('/invitations/') && request.method === 'DELETE') {
      state.invitations = [];
      return envelope({ Success: true, Data: invitation('ali@future.dev', 'developer') });
    }
    if (rest.endsWith('/resend')) return envelope({ Success: true, Data: { Invitation: invitation('ali@future.dev', 'developer'), Link: INVITE_LINK } });
    if (rest.startsWith('/members/')) {
      const userId = rest.split('/')[2] ?? '';
      return envelope({ Success: true, Data: { UserId: userId, Role: 'maintainer', Revoked: 2 } });
    }
    if (rest === '/transfer-ownership') return envelope({ Success: true, Data: { UserId: 'x', Role: 'owner', Revoked: 1 } });
    if (rest === '/leave') return envelope({ Success: true });
    return undefined;
  };
  return { calls, routes: [handler] };
};
