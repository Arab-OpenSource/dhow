import { createMemoryHistory } from '@tanstack/react-router';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { App } from '@/App/App';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { auditFixture, backendFailure, startBackend, type IBackendOptions } from '@/Core/Testing/Backend';
import { envelope } from '@/Core/Testing/Fetch';
import { Ar } from '@/I18n/Locales/Ar';

const mount = async (options: IBackendOptions = {}, url = '/?stack=audit') => {
  const backend = startBackend({ audit: auditFixture(45), ...options });
  const router = createAppRouter(createMemoryHistory({ initialEntries: [url] }));
  const user = userEvent.setup();
  render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
  const dialog = await screen.findByRole('dialog', { name: Ar['audit.title'] });
  return { ...backend, router, user, dialog };
};

const verifyButton = () => screen.findByRole('button', { name: Ar['audit.verify.action'] });

describe('the audit log slide', () => {
  it('lists events with what happened, who did it, from which address and when', async () => {
    const { dialog } = await mount();
    const list = await within(dialog).findByRole('list', { name: Ar['audit.list.title'] });
    const rows = within(list).getAllByRole('listitem');
    expect(rows).toHaveLength(20);
    const first = rows[0];
    expect(first).toHaveTextContent(Ar['audit.action.auth.login']);
    expect(first).toHaveTextContent('Sabry Dawood');
    expect(first).toHaveTextContent('203.0.113.1');
    expect(first).toHaveTextContent('من 10 دقيقة');
    expect(rows[2]).toHaveTextContent(Ar['audit.action.auth.login_failed']);
    expect(rows[2]).toHaveTextContent(Ar['audit.actor.visitor']);
    expect(within(dialog).getByText(Ar['audit.list.total'].replace('{count}', '45'))).toBeInTheDocument();
  });

  it('asks the API for one page at a time and moves between pages', async () => {
    const { dialog, user, calls } = await mount();
    await within(dialog).findByRole('list', { name: Ar['audit.list.title'] });
    expect(within(dialog).getByText(Ar['pager.status'].replace('{page}', '1').replace('{pages}', '3'))).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: Ar['pager.next'] }));
    await waitFor(() => {
      expect(within(dialog).getByText(Ar['pager.status'].replace('{page}', '2').replace('{pages}', '3'))).toBeInTheDocument();
    });
    const urls = calls.map((call) => new URL(call.url)).filter((url) => url.pathname.endsWith('/audit-events'));
    expect(urls.map((url) => [url.searchParams.get('Page'), url.searchParams.get('Limit')])).toEqual([
      ['1', '20'],
      ['2', '20'],
    ]);
    await user.click(within(dialog).getByRole('button', { name: Ar['pager.next'] }));
    await waitFor(() => {
      expect(within(dialog).getByText(Ar['pager.status'].replace('{page}', '3').replace('{pages}', '3'))).toBeInTheDocument();
    });
    expect(within(dialog).getByRole('button', { name: Ar['pager.next'] })).toBeDisabled();
    expect(within(await within(dialog).findByRole('list', { name: Ar['audit.list.title'] })).getAllByRole('listitem')).toHaveLength(5);
  });

  it('shows an empty log in words', async () => {
    await mount({ audit: [] });
    expect(await screen.findByText(Ar['audit.empty'])).toBeInTheDocument();
  });

  it('names an action it does not know in its raw form instead of hiding it', async () => {
    const [event] = auditFixture(1);
    await mount({ audit: event ? [{ ...event, Action: 'billing.invoice.paid' }] : [] });
    expect(await screen.findByText('billing.invoice.paid')).toBeInTheDocument();
  });

  it('shows an error with a retry when the log cannot be loaded', async () => {
    let failing = true;
    const { user } = await mount({
      routes: [
        (_request, url) =>
          failing && url.pathname.endsWith('/audit-events') ? backendFailure('INTERNAL', 'حصل خطأ في السيرفر.', 500) : undefined,
      ],
    });
    expect(await screen.findByText(Ar['audit.error.title'])).toBeInTheDocument();
    failing = false;
    await user.click(screen.getByRole('button', { name: Ar['status.retry'] }));
    expect(await screen.findByRole('list', { name: Ar['audit.list.title'] })).toBeInTheDocument();
  });
});

describe('verifying the chain', () => {
  it('says the chain was not checked yet, then that it is intact with the number of events', async () => {
    const { user, calls } = await mount();
    expect(await screen.findByText(Ar['audit.verify.idle.title'])).toBeInTheDocument();
    await user.click(await verifyButton());
    expect(await screen.findByText(Ar['audit.verify.intact.title'])).toBeInTheDocument();
    expect(screen.getByText(Ar['audit.verify.intact.sub'].replace('{count}', '45'))).toBeInTheDocument();
    const verify = calls.find((call) => call.url.endsWith('/audit-events/verify'));
    expect(verify?.method).toBe('GET');
    expect(screen.queryByText(Ar['audit.verify.idle.title'])).not.toBeInTheDocument();
  });

  it('says the chain is broken and where it broke', async () => {
    const { user } = await mount({
      verify: () => envelope({ Success: true, Data: { Valid: false, Checked: 12, FirstBrokenId: '0198a000-0000-7000-8000-000000000013' } }),
    });
    await user.click(await verifyButton());
    expect(await screen.findByText(Ar['audit.verify.broken.title'])).toBeInTheDocument();
    expect(screen.getByText(/اتفحص 12 حدث/)).toBeInTheDocument();
    expect(screen.getByText('0198a000-0000-7000-8000-000000000013')).toBeInTheDocument();
    expect(screen.queryByText(Ar['audit.verify.intact.title'])).not.toBeInTheDocument();
  });

  it('shows a failed check and lets the person retry it', async () => {
    let failing = true;
    const { user } = await mount({
      verify: () => (failing ? backendFailure('INTERNAL', 'حصل خطأ في السيرفر.', 500) : envelope({ Success: true, Data: { Valid: true, Checked: 45 } })),
    });
    await user.click(await verifyButton());
    expect(await screen.findByText(Ar['audit.verify.error'])).toBeInTheDocument();
    failing = false;
    const dialog = screen.getByRole('dialog', { name: Ar['audit.title'] });
    await user.click(within(dialog).getByRole('button', { name: Ar['status.retry'], hidden: true }));
    expect(await screen.findByText(Ar['audit.verify.intact.title'])).toBeInTheDocument();
  });

  it('forgets an old result when the slide is closed and opened again', async () => {
    const { user, router } = await mount();
    await user.click(await verifyButton());
    await screen.findByText(Ar['audit.verify.intact.title']);
    await user.click(screen.getByRole('button', { name: Ar['slides.closeAll'] }));
    await waitFor(() => {
      expect(router.state.location.href).toBe('/');
    });
    await user.click(await screen.findByRole('link', { name: Ar['shell.nav.audit'] }));
    expect(await screen.findByText(Ar['audit.verify.idle.title'])).toBeInTheDocument();
  });
});

describe('who can open the audit log', () => {
  it.each(['owner', 'admin'])('shows the menu entry to the %s and opens the slide from it', async (role) => {
    const backend = startBackend({ role, audit: auditFixture(3) });
    const router = createAppRouter(createMemoryHistory({ initialEntries: ['/'] }));
    const user = userEvent.setup();
    render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
    expect(backend.state.role).toBe(role);
    await user.click(await screen.findByRole('link', { name: Ar['shell.nav.audit'] }));
    expect(await screen.findByRole('dialog', { name: Ar['audit.title'] })).toBeInTheDocument();
    expect(router.state.location.href).toBe('/?stack=audit');
  });

  it.each(['maintainer', 'developer', 'viewer'])('hides the menu entry from the %s', async (role) => {
    startBackend({ role });
    const router = createAppRouter(createMemoryHistory({ initialEntries: ['/'] }));
    render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
    await screen.findByRole('navigation', { name: Ar['shell.nav.label'] });
    expect(screen.queryByRole('link', { name: Ar['shell.nav.audit'] })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: Ar['shell.nav.audit'] })).not.toBeInTheDocument();
  });

  it('shows a forbidden state, and no request, when a developer opens the link', async () => {
    const { calls, dialog } = await mount({ role: 'developer' });
    expect(await within(dialog).findByText(Ar['audit.forbidden'])).toBeInTheDocument();
    expect(calls.some((call) => call.url.includes('/audit-events'))).toBe(false);
    expect(within(dialog).queryByRole('button', { name: Ar['audit.verify.action'] })).not.toBeInTheDocument();
  });

  it('shows the same forbidden state when the server answers 403 anyway', async () => {
    await mount({
      routes: [(_request, url) => (url.pathname.endsWith('/audit-events') ? backendFailure('FORBIDDEN', 'مش مسموح.', 403) : undefined)],
    });
    expect(await screen.findByText(Ar['audit.forbidden'])).toBeInTheDocument();
  });

  it('hides the command from the palette for a viewer', async () => {
    const { user } = await mount({ role: 'viewer' });
    await user.keyboard('{Control>}k{/Control}');
    await user.type(await screen.findByRole('combobox'), Ar['palette.command.audit']);
    expect(screen.queryByRole('option', { name: new RegExp(Ar['palette.command.audit']) })).not.toBeInTheDocument();
  });
});
