import type { QueryClient } from '@tanstack/react-query';
import { createMemoryHistory } from '@tanstack/react-router';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';

import { App } from '@/App/App';
import type { TFrameId } from '@/App/Panels';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { auditFixture, startBackend } from '@/Core/Testing/Backend';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Ar } from '@/I18n/Locales/Ar';

const ACCOUNT_BUDGET = 10;
const TOUR_BUDGET = 13;
const UUID = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/g;

const ACCOUNT_ENDPOINTS = [
  'GET /api/v1/me',
  'GET /api/v1/meta',
  'GET /api/v1/me/preferences',
  'GET /api/v1/me/mfa',
  'GET /api/v1/me/passkeys',
  'GET /api/v1/oidc/providers',
  'GET /api/v1/me/identities',
  'GET /api/v1/me/api-tokens',
  'GET /api/v1/sessions',
];

const endpointOf = (request: Request): string => `${request.method} ${new URL(request.url).pathname.replace(UUID, '{id}')}`;

const countsOf = (calls: readonly Request[]): Record<string, number> => {
  const counts: Record<string, number> = {};
  for (const call of calls) {
    const endpoint = endpointOf(call);
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

const open = async (view: ReturnType<typeof mount>,stack: TFrameId[], title: string, expected: readonly string[]) => {
  await view.router.navigate({ to: '/', search: { stack } });
  await screen.findByRole('dialog', { name: title });
  await settle(view.client, view.calls, expected);
};

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
});

describe('API requests per view', () => {
  it('opens the account slide cold with each endpoint requested once and within the budget', async () => {
    const view = mount('/?stack=account');
    await screen.findByRole('dialog', { name: Ar['account.title'] });
    await settle(view.client, view.calls, ACCOUNT_ENDPOINTS);

    expect(repeated(view.calls)).toEqual([]);
    expect(view.calls.length).toBeLessThanOrEqual(ACCOUNT_BUDGET);
  });

  it('moves between the main slides without requesting anything twice, even while a slide leaves', async () => {
    const view = mount('/');
    await screen.findByRole('main', { hidden: true });
    await settle(view.client, view.calls, ['GET /api/v1/me', 'GET /api/v1/meta']);

    await open(view, ['account'], Ar['account.title'], ACCOUNT_ENDPOINTS);
    await open(view, ['organization'], Ar['organization.title'], ['GET /api/v1/organizations/{id}', 'GET /api/v1/organizations/{id}/members']);
    await open(view, ['audit'], Ar['audit.title'], ['GET /api/v1/organizations/{id}/audit-events']);
    await open(view, ['appearance'], Ar['appearance.title'], []);
    await open(view, ['account'], Ar['account.title'], []);
    await open(view, ['organization'], Ar['organization.title'], []);

    expect(repeated(view.calls.filter((call) => call.method === 'GET'))).toEqual([]);
    expect(view.calls.length).toBeLessThanOrEqual(TOUR_BUDGET);
  });

  it('still asks for the sessions again right after one is revoked', async () => {
    const view = mount('/?stack=account');
    const dialog = await screen.findByRole('dialog', { name: Ar['account.title'] });
    await settle(view.client, view.calls, ACCOUNT_ENDPOINTS);
    const list = await within(dialog).findByRole('list', { name: Ar['account.sessions.title'] });
    const firefox = (await within(list).findByText('Firefox على Windows')).closest('li');
    if (!firefox) throw new Error('no row for Firefox');

    await view.user.click(within(firefox).getByRole('button', { name: Ar['sessions.revoke.aria'].replace('{device}', 'Firefox على Windows') }));
    await view.user.click(within(firefox).getByRole('button', { name: Ar['sessions.revoke.confirm'] }));

    await waitFor(() => {
      expect(countsOf(view.calls)['GET /api/v1/sessions']).toBe(2);
    });
    expect(within(list).queryByText('Firefox على Windows')).not.toBeInTheDocument();
  });
});
