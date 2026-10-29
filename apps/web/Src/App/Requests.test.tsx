import type { QueryClient } from '@tanstack/react-query';
import { createMemoryHistory } from '@tanstack/react-router';
import { configure, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';

import { App } from '@/App/App';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { auditFixture, startBackend } from '@/Core/Testing/Backend';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Ar } from '@/I18n/Locales/Ar';

configure({ asyncUtilTimeout: 5000 });

const UUID = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/g;
const SHELL = ['GET /api/v1/me', 'GET /api/v1/meta', 'GET /api/v1/me/preferences', 'GET /api/v1/health/ready'];
const ORG = 'GET /api/v1/organizations/{id}';

const endpointOf = (request: Request): string => `${request.method} ${new URL(request.url).pathname.replace(UUID, '{id}')}`;

const requestOf = (request: Request): string => `${endpointOf(request)}${new URL(request.url).search}`;

const countsOf = (calls: readonly Request[], key: (request: Request) => string = requestOf): Record<string, number> => {
  const counts: Record<string, number> = {};
  for (const call of calls) {
    const endpoint = key(call);
    counts[endpoint] = (counts[endpoint] ?? 0) + 1;
  }
  return counts;
};

const repeated = (calls: readonly Request[]): string[] =>
  Object.entries(countsOf(calls))
    .filter(([, count]) => count > 1)
    .map(([endpoint, count]) => `${endpoint} x${count}`);

const settle = async (client: QueryClient, calls: readonly Request[], expected: readonly string[]) => {
  for (let round = 0; round < 2; round += 1) {
    await waitFor(() => {
      expect(calls.map(endpointOf)).toEqual(expect.arrayContaining([...expected]));
      expect(client.isFetching()).toBe(0);
    });
    await new Promise((resolve) => setTimeout(resolve, 60));
  }
};

const mount = (url: string) => {
  const backend = startBackend({ audit: auditFixture(3) });
  const router = createAppRouter(createMemoryHistory({ initialEntries: [url] }));
  const client = createQueryClient({ retry: false });
  const user = userEvent.setup();
  render(<App router={router} queryClient={client} />);
  return { ...backend, router, client, user };
};

const ROUTES: readonly [string, string, readonly string[]][] = [
  ['/', Ar['home.title'], []],
  ['/jobs', Ar['jobs.title'], ['GET /api/v1/organizations/{id}/jobs']],
  ['/runners', Ar['runners.title'], ['GET /api/v1/organizations/{id}/runners']],
  ['/runners/tokens', Ar['runners.tokens.title'], ['GET /api/v1/organizations/{id}/runner-registration-tokens']],
  ['/org/members', Ar['shell.nav.members'], [ORG, 'GET /api/v1/organizations/{id}/members']],
  ['/org/teams', Ar['shell.nav.teams'], ['GET /api/v1/organizations/{id}/teams']],
  ['/org/invitations', Ar['shell.nav.invitations'], ['GET /api/v1/organizations/{id}/invitations']],
  ['/org/api-keys', Ar['shell.nav.apiKeys'], ['GET /api/v1/organizations/{id}/api-tokens']],
  ['/org/settings', Ar['shell.nav.orgSettings'], [ORG, 'GET /api/v1/me/mfa']],
  ['/audit', Ar['audit.title'], ['GET /api/v1/organizations/{id}/audit-events', 'GET /api/v1/organizations/{id}/members']],
  ['/system', Ar['status.title'], []],
  ['/account/profile', Ar['shell.nav.profile'], ['GET /api/v1/me/api-tokens']],
  [
    '/account/security',
    Ar['shell.nav.security'],
    ['GET /api/v1/me/mfa', 'GET /api/v1/me/passkeys', 'GET /api/v1/oidc/providers', 'GET /api/v1/me/identities', 'GET /api/v1/sessions'],
  ],
  ['/account/preferences', Ar['shell.nav.preferences'], []],
  ['/organizations', Ar['organizations.title'], []],
  ['/notifications', Ar['notifications.title'], []],
];

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
});

describe('API requests per page', () => {
  it.each(ROUTES)('opens %s cold with only what the page shows, each once', async (url, title, own) => {
    const view = mount(url);
    await screen.findByRole('heading', { level: 1, name: title });
    const expected = [...SHELL, ...own];
    await settle(view.client, view.calls, expected);

    expect(repeated(view.calls)).toEqual([]);
    expect([...new Set(view.calls.map(endpointOf))].sort()).toEqual([...expected].sort());
  });

  it('moves between pages without asking for anything twice', async () => {
    const view = mount('/');
    await screen.findByRole('heading', { level: 1, name: Ar['home.title'] });
    await settle(view.client, view.calls, SHELL);
    const nav = screen.getByRole('navigation', { name: Ar['shell.nav.label'] });

    for (const [label, title] of [
      [Ar['shell.nav.members'], Ar['shell.nav.members']],
      [Ar['shell.nav.audit'], Ar['audit.title']],
      [Ar['shell.nav.orgSettings'], Ar['shell.nav.orgSettings']],
      [Ar['shell.nav.system'], Ar['status.title']],
      [Ar['shell.nav.members'], Ar['shell.nav.members']],
      [Ar['shell.nav.home'], Ar['home.title']],
    ] as const) {
      await view.user.click(within(nav).getByRole('link', { name: label }));
      await screen.findByRole('heading', { level: 1, name: title });
      await settle(view.client, view.calls, []);
    }

    expect(repeated(view.calls.filter((call) => call.method === 'GET'))).toEqual([]);
  });

  it('still asks for the sessions again right after one is revoked', async () => {
    const view = mount('/account/security');
    await screen.findByRole('heading', { level: 1, name: Ar['shell.nav.security'] });
    await settle(view.client, view.calls, ['GET /api/v1/sessions']);
    const list = await screen.findByRole('list', { name: Ar['account.sessions.title'] });
    const firefox = (await within(list).findByText('Firefox على Windows')).closest('li');
    if (!firefox) throw new Error('no row for Firefox');

    await view.user.click(within(firefox).getByRole('button', { name: Ar['sessions.revoke.aria'].replace('{device}', 'Firefox على Windows') }));
    await view.user.click(within(firefox).getByRole('button', { name: Ar['sessions.revoke.confirm'] }));

    await waitFor(() => {
      expect(countsOf(view.calls, endpointOf)['GET /api/v1/sessions']).toBe(2);
    });
    expect(within(list).queryByText('Firefox على Windows')).not.toBeInTheDocument();
  });
});
