import { createMemoryHistory } from '@tanstack/react-router';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { App } from '@/App/App';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { auditFixture, backendFailure, startBackend, type IBackendOptions, type TRouteHandler } from '@/Core/Testing/Backend';
import { envelope, SESSION_ID } from '@/Core/Testing/Fetch';
import { findPage } from '@/Core/Testing/Pages';
import { Ar } from '@/I18n/Locales/Ar';

const MONA = '0198a000-0000-7000-8000-0000000000b2';
const EVENTS = auditFixture(5);
const FIRST = EVENTS[0]?.Id ?? '';

const oneEvent =
  (reply: (id: string) => ReturnType<TRouteHandler>): TRouteHandler =>
  (_request, url) => {
    const match = /\/audit-events\/([0-9a-f-]{36})$/.exec(url.pathname);
    return match?.[1] ? reply(match[1]) : undefined;
  };

const found = oneEvent((id) => {
  const event = EVENTS.find((item) => item.Id === id);
  return event ? envelope({ Success: true, Data: { ...event, Data: { Via: 'api_token', ViaId: 'tok-1', Reason: 'rotation' } } }) : undefined;
});

const mount = async (options: IBackendOptions = {}, url = '/audit', title: string = Ar['audit.title']) => {
  const backend = startBackend({ audit: EVENTS, ...options });
  const router = createAppRouter(createMemoryHistory({ initialEntries: [url] }));
  const user = userEvent.setup();
  render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
  const dialog = await findPage(title);
  return { ...backend, router, user, dialog };
};

const auditQueries = (calls: readonly Request[]) =>
  calls.map((call) => new URL(call.url)).filter((url) => url.pathname.endsWith('/audit-events'));

describe('filtering the audit log', () => {
  it('sends the type, person and dates to the API and starts from the first page', async () => {
    const { dialog, user, calls } = await mount();
    await within(dialog).findByRole('list', { name: Ar['audit.list.title'] });
    const filters = within(dialog).getByRole('search', { name: Ar['audit.filter.title'] });
    await user.selectOptions(within(filters).getByLabelText(Ar['audit.filter.category']), 'auth');
    await user.selectOptions(await within(filters).findByLabelText(Ar['audit.filter.actor']), MONA);
    await user.type(within(filters).getByLabelText(Ar['audit.filter.from']), '2026-10-01');
    await user.type(within(filters).getByLabelText(Ar['audit.filter.to']), '2026-10-02');
    await waitFor(() => {
      const last = auditQueries(calls).at(-1);
      expect(last?.searchParams.get('To')).not.toBeNull();
    });
    const last = auditQueries(calls).at(-1);
    expect(last?.searchParams.get('Category')).toBe('auth');
    expect(last?.searchParams.get('ActorUserId')).toBe(MONA);
    expect(last?.searchParams.get('Page')).toBe('1');
    const from = new Date(last?.searchParams.get('From') ?? '');
    const to = new Date(last?.searchParams.get('To') ?? '');
    expect(to.getTime() - from.getTime()).toBe(2 * 24 * 3_600_000);
    expect(from.getDate()).toBe(1);
  });

  it('does not ask the API while the start date is after the end date', async () => {
    const { dialog, user, calls } = await mount();
    await within(dialog).findByRole('list', { name: Ar['audit.list.title'] });
    const filters = within(dialog).getByRole('search', { name: Ar['audit.filter.title'] });
    await user.type(within(filters).getByLabelText(Ar['audit.filter.to']), '2026-10-01');
    const before = auditQueries(calls).length;
    await user.type(within(filters).getByLabelText(Ar['audit.filter.from']), '2026-10-05');
    expect(await within(filters).findByText(Ar['audit.filter.range'])).toBeInTheDocument();
    expect(auditQueries(calls).filter((url) => url.searchParams.get('From') !== null)).toHaveLength(0);
    expect(auditQueries(calls).length).toBe(before);
  });

  it('says when nothing matches, and clears the filters', async () => {
    const { dialog, user } = await mount({
      routes: [(_request, url) => (url.searchParams.get('Category') === 'job' ? envelope({ Success: true, Data: [], Meta: { Page: 1, Limit: 20, Total: 0 } }) : undefined)],
    });
    await within(dialog).findByRole('list', { name: Ar['audit.list.title'] });
    await user.selectOptions(within(dialog).getByLabelText(Ar['audit.filter.category']), 'job');
    expect(await within(dialog).findByText(Ar['audit.filter.empty'])).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: Ar['audit.filter.clear'] }));
    expect(await within(dialog).findByRole('list', { name: Ar['audit.list.title'] })).toBeInTheDocument();
    expect(within(dialog).getByLabelText(Ar['audit.filter.category'])).toHaveValue('');
  });
});

describe('an audit event in detail', () => {
  it('opens from its row and shows who, what, when, where, the target and the data', async () => {
    const { dialog, user, router } = await mount({ routes: [found] });
    const list = await within(dialog).findByRole('list', { name: Ar['audit.list.title'] });
    const [first] = within(list).getAllByRole('button');
    if (first === undefined) throw new Error('no row');
    await user.click(first);
    const detail = await findPage(Ar['audit.detail.title']);
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/audit/${FIRST}`);
    });
    expect(await within(detail).findByRole('heading', { name: Ar['audit.action.auth.login'] })).toBeInTheDocument();
    const facts = within(detail).getByText(Ar['audit.detail.who']).closest('dl');
    if (!(facts instanceof HTMLElement)) throw new Error('no facts');
    expect(within(facts).getByText('Sabry Dawood')).toBeInTheDocument();
    expect(within(facts).getByText('203.0.113.1')).toBeInTheDocument();
    expect(within(facts).getByText(Ar['audit.target.session'])).toBeInTheDocument();
    expect(within(facts).getByText(SESSION_ID)).toBeInTheDocument();
    expect(within(facts).getByText(Ar['audit.detail.via.token'])).toBeInTheDocument();
    expect(within(detail).getByText('Reason')).toBeInTheDocument();
    expect(within(detail).getByText('rotation')).toBeInTheDocument();
  });

  it('shows a loading state, then says when the event is not in this organization', async () => {
    await mount(
      { routes: [oneEvent(() => backendFailure('NOT_FOUND', 'x', 404))] },
      `/audit/${FIRST}`,
      Ar['audit.detail.title'],
    );
    expect(await screen.findByText(Ar['audit.detail.missing'])).toBeInTheDocument();
  });

  it('shows an error with a retry', async () => {
    let failing = true;
    const { user } = await mount(
      { routes: [oneEvent((id) => (failing ? backendFailure('INTERNAL', 'حدث خطأ في السيرفر.', 500) : found(new Request(`http://x/audit-events/${id}`), new URL(`http://x/audit-events/${id}`))))] },
      `/audit/${FIRST}`,
      Ar['audit.detail.title'],
    );
    expect(await screen.findByText(Ar['audit.detail.error'])).toBeInTheDocument();
    failing = false;
    await user.click(screen.getByRole('button', { name: Ar['status.retry'] }));
    expect(await screen.findByText('rotation')).toBeInTheDocument();
  });
});
