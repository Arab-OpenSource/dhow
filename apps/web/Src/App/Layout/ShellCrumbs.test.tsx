import { createMemoryHistory } from '@tanstack/react-router';
import { configure, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { App } from '@/App/App';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { useCrumbStore } from '@/Core/Store/useCrumbStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { auditFixture, startBackend, type TRouteHandler } from '@/Core/Testing/Backend';
import { envelope } from '@/Core/Testing/Fetch';
import { gate } from '@/Core/Testing/Gate';
import { JOB_ID, jobFixture, jobsRoute } from '@/Core/Testing/JobsBackend';
import { orgBackend, TEAM_ID } from '@/Core/Testing/OrgBackend';
import { runnerFixture, runnerId, runnersRoute } from '@/Core/Testing/RunnersBackend';
import { Ar } from '@/I18n/Locales/Ar';
import type { TMessageKey } from '@/I18n/Messages';

configure({ asyncUtilTimeout: 5000 });

const HANA = '0198a000-0000-7000-8000-0000000000b3';
const MISSING = '0198a000-0000-7000-8000-0000000000f9';
const EVENT = auditFixture(1)[0];

const auditEventRoute: TRouteHandler = (request, url) =>
  request.method === 'GET' && EVENT !== undefined && url.pathname.endsWith(`/audit-events/${EVENT.Id}`)
    ? envelope({ Success: true, Data: EVENT })
    : undefined;

const mount = (url: string, routes: TRouteHandler[] = []) => {
  startBackend({
    role: 'owner',
    audit: auditFixture(1),
    routes: [
      ...routes,
      jobsRoute({ jobs: [jobFixture({ Name: 'deploy-staging' })] }),
      runnersRoute({ runners: [runnerFixture()] }),
      auditEventRoute,
      ...orgBackend().routes,
    ],
  });
  const router = createAppRouter(createMemoryHistory({ initialEntries: [url] }));
  render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
  return router;
};

const crumbs = () =>
  within(screen.getByRole('navigation', { name: Ar['shell.crumbs.label'] }))
    .getAllByRole('listitem')
    .map((item) => item.textContent);

const lastCrumb = async (label: string) => {
  await waitFor(() => {
    expect(crumbs().at(-1)).toBe(label);
  });
  const current = within(screen.getByRole('navigation', { name: Ar['shell.crumbs.label'] })).getByText(label);
  expect(current.tagName).toBe('BDI');
};

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
  useCrumbStore.setState({ Titles: {} });
});

describe('the top bar path of an item', { timeout: 15_000 }, () => {
  it.each([
    ['a job', `/jobs/${JOB_ID}`, 'deploy-staging'],
    ['a runner', `/runners/${runnerId(1)}`, 'build-01'],
    ['a member', `/org/members/${HANA}`, 'Hana Samir'],
    ['a team', `/org/teams/${TEAM_ID}`, 'Backend'],
    ['an audit event', `/audit/${EVENT?.Id ?? ''}`, Ar['audit.action.auth.login']],
  ])('names %s', async (_kind, url, name) => {
    mount(url);
    await lastCrumb(name);
  });

  it.each<[string, string, TMessageKey]>([
    ['a job', `/jobs/${MISSING}`, 'jobs.detail.title'],
    ['a runner', `/runners/${MISSING}`, 'runners.detail.title'],
    ['a member', `/org/members/${MISSING}`, 'shell.crumb.member'],
    ['a team', `/org/teams/${MISSING}`, 'shell.crumb.team'],
    ['an audit event', `/audit/${MISSING}`, 'audit.detail.title'],
  ])('keeps the generic label for %s that is not found', async (_kind, url, key) => {
    mount(url, [
      (request, requested) =>
        request.method === 'GET' && requested.pathname.endsWith(MISSING)
          ? envelope({ Success: false, Error: { Code: 'NOT_FOUND', Message: 'x' } }, 404)
          : undefined,
    ]);
    await screen.findByRole('navigation', { name: Ar['shell.crumbs.label'] });
    await new Promise((resolve) => setTimeout(resolve, 200));
    expect(crumbs().at(-1)).toBe(Ar[key]);
  });

  it('keeps the generic label while the item loads', async () => {
    mount(`/jobs/${JOB_ID}`, [gate(/\/jobs\/[0-9a-f-]{36}$/, 'hang').route]);
    expect(await screen.findByRole('status', { name: Ar['jobs.detail.loading'] })).toBeInTheDocument();
    expect(crumbs().at(-1)).toBe(Ar['jobs.detail.title']);
  });

  it('forgets the name when the item page closes', async () => {
    const router = mount(`/jobs/${JOB_ID}`);
    await lastCrumb('deploy-staging');
    expect(useCrumbStore.getState().Titles).toEqual({ [JOB_ID]: 'deploy-staging' });
    await router.navigate({ to: '/jobs' });
    await waitFor(() => {
      expect(useCrumbStore.getState().Titles).toEqual({});
    });
    expect(crumbs().at(-1)).toBe(Ar['shell.nav.jobs']);
  });
});
