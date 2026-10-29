import { screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { auditFixture, startBackend, type IAuditFixture } from '@/Core/Testing/Backend';
import { ORG_ID } from '@/Core/Testing/Fetch';
import { gate, type TGateMode } from '@/Core/Testing/Gate';
import { renderWithProviders } from '@/Core/Testing/Render';
import { Ar } from '@/I18n/Locales/Ar';
import { RecentEventsCard } from '@/Modules/Audit/Components/RecentEventsCard';

const mount = (audit: IAuditFixture[], mode: TGateMode = 'pass') => {
  const auditGate = gate(/\/audit-events$/, mode);
  const backend = startBackend({ audit, routes: [auditGate.route] });
  const onOpenEvent = vi.fn();
  const onShowAll = vi.fn();
  const view = renderWithProviders(<RecentEventsCard organizationId={ORG_ID} onOpenEvent={onOpenEvent} onShowAll={onShowAll} />);
  return { ...view, ...backend, auditGate, onOpenEvent, onShowAll };
};

describe('RecentEventsCard', () => {
  it('shows a skeleton while the events load', async () => {
    mount(auditFixture(2), 'hang');
    expect(await screen.findByRole('status', { name: Ar['home.events.loading'] })).toBeInTheDocument();
  });

  it('shows an error and loads again on retry', async () => {
    const { auditGate, user } = mount(auditFixture(2), 'fail');
    expect(await screen.findByText(Ar['home.events.error'])).toBeInTheDocument();
    auditGate.set('pass');
    await user.click(screen.getByRole('button', { name: Ar['jobs.retry'] }));
    expect(await screen.findByRole('list', { name: Ar['home.events.title'] })).toBeInTheDocument();
  });

  it('says so when the log is empty', async () => {
    mount([]);
    expect(await screen.findByText(Ar['home.events.empty'])).toBeInTheDocument();
  });

  it('lists the five newest events with who did them', async () => {
    const { calls, user, onOpenEvent, onShowAll } = mount(auditFixture(9));
    const list = await screen.findByRole('list', { name: Ar['home.events.title'] });
    expect(within(list).getAllByRole('listitem')).toHaveLength(5);
    expect((await within(list).findAllByText(/Sabry Dawood/)).length).toBeGreaterThan(0);
    const asked = calls.map((call) => new URL(call.url)).filter((url) => url.pathname.endsWith('/audit-events'));
    expect(asked.map((url) => url.search)).toEqual(['?Page=1&Limit=5']);
    await user.click(within(list).getAllByRole('button')[0] ?? list);
    expect(onOpenEvent).toHaveBeenCalledWith('0198a000-0000-7000-8000-000000000001');
    await user.click(screen.getByRole('button', { name: Ar['home.events.all'] }));
    expect(onShowAll).toHaveBeenCalledOnce();
  });
});
