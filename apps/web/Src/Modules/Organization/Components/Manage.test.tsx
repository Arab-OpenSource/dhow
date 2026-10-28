import { createMemoryHistory } from '@tanstack/react-router';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';

import { App } from '@/App/App';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { formatDateTime } from '@/Core/Format/TimeFormat';
import { useOrganizationStore } from '@/Core/Store/useOrganizationStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { membersFixture, startBackend, type IBackendOptions } from '@/Core/Testing/Backend';
import { envelope, meBody, ORG_ID } from '@/Core/Testing/Fetch';
import { INVITE_LINK, orgBackend, TEAM_ID } from '@/Core/Testing/OrgBackend';
import { Ar } from '@/I18n/Locales/Ar';
import { useOrganizationTabStore } from '@/Modules/Organization/Store/useOrganizationTabStore';

const HANA = '0198a000-0000-7000-8000-0000000000b3';
const MONA = '0198a000-0000-7000-8000-0000000000b2';
const OTHER_ORG = '0198a000-0000-7000-8000-0000000000a2';

const open = async (url: string, title: string, options: IBackendOptions = {}, emailStatus?: string) => {
  const org = orgBackend(emailStatus);
  const backend = startBackend({ ...options, routes: [...(options.routes ?? []), ...org.routes] });
  const router = createAppRouter(createMemoryHistory({ initialEntries: [url] }));
  const user = userEvent.setup();
  render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
  const dialog = await screen.findByRole('dialog', { name: title });
  return { ...backend, org, router, user, dialog };
};

const bodyOf = async (calls: Request[], method: string, suffix: string): Promise<unknown> => {
  const call = calls.find((request) => request.method === method && new URL(request.url).pathname.endsWith(suffix));
  return call ? ((await call.clone().json()) as unknown) : undefined;
};

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
  useOrganizationTabStore.setState({ Tab: 'members' });
  useOrganizationStore.setState({ CurrentId: null });
});

describe('inviting a member', () => {
  it('offers only roles up to the inviter, then shows the link once and says no email went out', async () => {
    const { user, dialog, calls } = await open('/?stack=organization,organization-invite', Ar['invite.title'], { role: 'admin' });
    const roles = within(dialog).getByRole('radiogroup', { name: Ar['invite.role'] });
    expect(within(roles).queryByRole('radio', { name: new RegExp(Ar['role.owner']) })).not.toBeInTheDocument();
    expect(within(roles).getAllByRole('radio')).toHaveLength(4);
    await user.type(within(dialog).getByLabelText(Ar['invite.email']), 'new@future.dev');
    await user.click(within(roles).getByRole('radio', { name: new RegExp(Ar['role.viewer']) }));
    await user.click(within(dialog).getByRole('button', { name: Ar['invite.create'] }));
    expect(await within(dialog).findByText(INVITE_LINK)).toBeInTheDocument();
    expect(within(dialog).getByText(Ar['invite.email.disabled'])).toBeInTheDocument();
    expect(await bodyOf(calls, 'POST', '/invitations')).toEqual({ Email: 'new@future.dev', Role: 'viewer' });
  });

  it('refuses a malformed email without calling the server', async () => {
    const { user, dialog, calls } = await open('/?stack=organization,organization-invite', Ar['invite.title']);
    await user.type(within(dialog).getByLabelText(Ar['invite.email']), 'not-an-email');
    await user.click(within(dialog).getByRole('button', { name: Ar['invite.create'] }));
    expect(await within(dialog).findByText(Ar['form.error.email'])).toBeInTheDocument();
    expect(calls.some((call) => call.method === 'POST')).toBe(false);
  });

  it('says when the email was sent', async () => {
    const { user, dialog } = await open('/?stack=organization,organization-invite', Ar['invite.title'], {}, 'sent');
    await user.type(within(dialog).getByLabelText(Ar['invite.email']), 'new@future.dev');
    await user.click(within(dialog).getByRole('button', { name: Ar['invite.create'] }));
    expect(await within(dialog).findByText(Ar['invite.email.sent'].replace('{email}', 'new@future.dev'))).toBeInTheDocument();
  });

  it('lists open invitations, revokes one and makes a new link for another', async () => {
    useOrganizationTabStore.setState({ Tab: 'invitations' });
    const { user, dialog, calls } = await open('/?stack=organization', Ar['organization.title']);
    const list = await within(dialog).findByRole('list', { name: Ar['invitations.title'] });
    expect(list).toHaveTextContent('ali@future.dev');
    expect(list).toHaveTextContent(Ar['invitations.expires'].replace('{when}', formatDateTime('2026-11-03T08:00:00Z', 'ar')));
    await user.click(within(list).getByRole('button', { name: Ar['invitations.resendLabel'].replace('{email}', 'ali@future.dev') }));
    expect(await within(dialog).findByText(INVITE_LINK)).toBeInTheDocument();
    await user.click(within(list).getByRole('button', { name: Ar['invitations.revokeLabel'].replace('{email}', 'ali@future.dev') }));
    expect(await within(dialog).findByText(Ar['invitations.empty'])).toBeInTheDocument();
    expect(calls.some((call) => call.method === 'DELETE' && call.url.includes('/invitations/'))).toBe(true);
  });
});

describe('a member slide', () => {
  it('changes a role and says the sessions ended', async () => {
    const { user, dialog, calls } = await open(`/?stack=organization,member-${HANA}`, Ar['organization.tab.members']);
    const roles = await within(dialog).findByRole('radiogroup', { name: Ar['member.role.title'] });
    await user.click(within(roles).getByRole('radio', { name: new RegExp(Ar['role.maintainer']) }));
    await user.click(within(dialog).getByRole('button', { name: Ar['member.role.save'] }));
    await waitFor(() => {
      expect(useToastStore.getState().Toasts.some((toast) => toast.Title.includes('Hana Samir'))).toBe(true);
    });
    expect(await bodyOf(calls, 'PATCH', `/members/${HANA}`)).toEqual({ Role: 'maintainer' });
  });

  it('asks for a current factor before granting the owner role', async () => {
    const { user, dialog, calls } = await open(`/?stack=organization,member-${HANA}`, Ar['organization.tab.members']);
    const roles = await within(dialog).findByRole('radiogroup', { name: Ar['member.role.title'] });
    await user.click(within(roles).getByRole('radio', { name: new RegExp(Ar['role.owner']) }));
    await user.click(within(dialog).getByRole('button', { name: Ar['member.role.save'] }));
    await user.type(await within(dialog).findByLabelText(Ar['factor.method.password']), 'correct horse battery staple');
    await user.click(within(dialog).getAllByRole('button', { name: Ar['member.role.save'] }).at(-1) ?? dialog);
    await waitFor(async () => {
      expect(await bodyOf(calls, 'PATCH', `/members/${HANA}`)).toEqual({
        Role: 'owner',
        CurrentFactor: { Method: 'password', Password: 'correct horse battery staple' },
      });
    });
  });

  it('resets a lost second factor after the actor confirms', async () => {
    const { user, dialog, calls } = await open(`/?stack=organization,member-${MONA}`, Ar['organization.tab.members']);
    await user.click(await within(dialog).findByRole('button', { name: Ar['member.mfa.reset'] }));
    await user.type(await within(dialog).findByLabelText(Ar['factor.method.password']), 'correct horse battery staple');
    await user.click(within(dialog).getAllByRole('button', { name: Ar['member.mfa.reset'] }).at(-1) ?? dialog);
    await waitFor(async () => {
      expect(await bodyOf(calls, 'POST', `/members/${MONA}/mfa-reset`)).toEqual({
        CurrentFactor: { Method: 'password', Password: 'correct horse battery staple' },
      });
    });
  });

  it('sends an owner through the role change before removing them', async () => {
    const coOwner = '0198a000-0000-7000-8000-0000000000b4';
    const members = [
      ...membersFixture(),
      { UserId: coOwner, Name: 'Laila Owner', Email: 'laila@future.dev', Role: 'owner', EffectiveRole: 'owner', JoinedAt: '2026-10-06T08:00:00Z', Mfa: 'totp' },
    ];
    const { dialog } = await open(`/?stack=organization,member-${coOwner}`, Ar['organization.tab.members'], { members });
    expect(await within(dialog).findByText(Ar['member.remove.ownerFirst'].replace('{name}', 'Laila Owner'))).toBeInTheDocument();
    expect(within(dialog).queryByRole('button', { name: Ar['member.remove.action'].replace('{name}', 'Laila Owner') })).not.toBeInTheDocument();
  });

  it('points to My account on your own member slide', async () => {
    const owner = '0198a000-0000-7000-8000-0000000000b1';
    const { dialog } = await open(`/?stack=organization,member-${owner}`, Ar['organization.tab.members'], { role: 'admin' });
    expect(await within(dialog).findByText(Ar['member.self'])).toBeInTheDocument();
  });
});

describe('teams', () => {
  it('opens a team, raises its role and reports the ended sessions', async () => {
    const { user, dialog, calls } = await open(`/?stack=organization,team-${TEAM_ID}`, Ar['organization.tab.teams']);
    const roles = await within(dialog).findByRole('radiogroup', { name: Ar['team.role'] });
    expect(within(dialog).getByText('Hana Samir')).toBeInTheDocument();
    await user.click(within(roles).getByRole('radio', { name: new RegExp(Ar['role.admin']) }));
    await user.click(within(dialog).getByRole('button', { name: Ar['team.role.save'] }));
    await waitFor(() => {
      expect(useToastStore.getState().Toasts.some((toast) => toast.Detail === Ar['team.revoked'].replace('{count}', '1'))).toBe(true);
    });
    expect(await bodyOf(calls, 'PATCH', '/role-bindings/0198a000-0000-7000-8000-0000000000d2')).toEqual({ Role: 'admin' });
  });

  it('lists teams with their organization role', async () => {
    useOrganizationTabStore.setState({ Tab: 'teams' });
    const { dialog } = await open('/?stack=organization', Ar['organization.title']);
    const list = await within(dialog).findByRole('list', { name: Ar['teams.title'] });
    expect(list).toHaveTextContent('Backend');
    expect(list).toHaveTextContent(Ar['teams.roleIn'].replace('{role}', Ar['role.developer']));
  });
});

describe('switching and leaving organizations', () => {
  const twoOrgs = (): IBackendOptions => ({
    routes: [
      (request, url) => {
        if (!url.pathname.endsWith('/me') || request.method !== 'GET') return undefined;
        const me = meBody();
        me.Data.Memberships.push({ OrganizationId: OTHER_ORG, OrganizationName: 'Sweets Store', OrganizationSlug: 'sweets', Role: 'viewer' });
        return envelope(me);
      },
    ],
  });

  it('switches through the organization chip and remembers the choice', async () => {
    const { user, dialog } = await open('/?stack=organizations', Ar['organizations.title'], twoOrgs());
    expect(within(dialog).getByText(Ar['organizations.current'])).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: /Sweets Store/ }));
    await waitFor(() => {
      expect(useOrganizationStore.getState().CurrentId).toBe(OTHER_ORG);
    });
    expect(
      (await screen.findAllByRole('button', { name: Ar['shell.organization.switch'].replace('{name}', 'Sweets Store') })).length,
    ).toBeGreaterThan(0);
  });

  it('leaves after a confirmation', async () => {
    useOrganizationTabStore.setState({ Tab: 'settings' });
    const { user, dialog, calls } = await open('/?stack=organization', Ar['organization.title'], { role: 'developer' });
    await user.click(await within(dialog).findByRole('button', { name: Ar['organization.leave.action'].replace('{name}', 'Future Solution') }));
    await user.click(within(dialog).getByRole('button', { name: Ar['organization.leave.confirm'] }));
    await waitFor(() => {
      expect(calls.some((call) => call.method === 'POST' && call.url.endsWith(`/organizations/${ORG_ID}/leave`))).toBe(true);
    });
  });

  it('hands the ownership over with a factor', async () => {
    const { user, dialog, calls } = await open('/?stack=organization,organization-transfer', Ar['organization.transfer.title']);
    await user.click(await within(dialog).findByRole('radio', { name: /Mona Adel/ }));
    await user.type(await within(dialog).findByLabelText(Ar['factor.method.password']), 'correct horse battery staple');
    await user.click(within(dialog).getByRole('button', { name: Ar['organization.transfer.action'] }));
    await waitFor(async () => {
      expect(await bodyOf(calls, 'POST', '/transfer-ownership')).toEqual({
        UserId: MONA,
        CurrentFactor: { Method: 'password', Password: 'correct horse battery staple' },
      });
    });
  });
});

describe('the second-step reset banner', () => {
  it('asks the member to set up a new factor', async () => {
    const backend = startBackend({
      routes: [
        (request, url) => {
          if (!url.pathname.endsWith('/me') || request.method !== 'GET') return undefined;
          const me = meBody();
          me.Data.MfaReenrollRequired = true;
          return envelope(me);
        },
      ],
    });
    const router = createAppRouter(createMemoryHistory({ initialEntries: ['/'] }));
    const user = userEvent.setup();
    render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
    expect(await screen.findByText(Ar['mfaReset.banner.title'])).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: Ar['mfaReset.banner.action'] }));
    expect(await screen.findByRole('dialog', { name: Ar['account.title'] })).toBeInTheDocument();
    expect(backend.state.signedIn).toBe(true);
  });
});
