import { screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { envelope, ORG_ID } from '@/Core/Testing/Fetch';
import { startBackend } from '@/Core/Testing/Backend';
import { gate, type TGateMode } from '@/Core/Testing/Gate';
import { renderWithProviders } from '@/Core/Testing/Render';
import { Ar } from '@/I18n/Locales/Ar';
import { RunnersSummaryCard } from '@/Modules/Runners/Components/RunnersSummaryCard';

const runner = (index: number, seen: boolean) => ({
  Id: `0198a000-0000-7000-8000-0000000001${String(index).padStart(2, '0')}`,
  OrganizationId: ORG_ID,
  Name: `runner-${String(index)}`,
  Labels: ['linux'],
  CapabilityLabels: [],
  Os: 'linux',
  Arch: 'amd64',
  Version: '0.4.0',
  IsolationLevels: ['container'],
  Capacity: 2,
  State: 'active',
  Status: seen ? 'online' : 'offline',
  KeyVersion: 1,
  KeysRotatedAt: '2026-10-20T08:00:00Z',
  LastSeenAt: seen ? new Date().toISOString() : undefined,
  CreatedAt: '2026-10-20T08:00:00Z',
});

const mount = (count: number, mode: TGateMode = 'pass', onAddRunner?: () => void) => {
  const runnersGate = gate(/\/runners$/, mode);
  const list = Array.from({ length: count }, (_, index) => runner(index, index % 2 === 0));
  const backend = startBackend({
    routes: [
      runnersGate.route,
      (_request, url) => {
        if (!url.pathname.endsWith('/runners')) return undefined;
        const limit = Number(url.searchParams.get('Limit'));
        return envelope({ Success: true, Data: list.slice(0, limit), Meta: { Page: 1, Limit: limit, Total: list.length } });
      },
    ],
  });
  const onOpenRunner = vi.fn();
  const onShowAll = vi.fn();
  const view = renderWithProviders(
    <RunnersSummaryCard organizationId={ORG_ID} onOpenRunner={onOpenRunner} onShowAll={onShowAll} onAddRunner={onAddRunner} />,
  );
  return { ...view, ...backend, runnersGate, onOpenRunner, onShowAll };
};

describe('RunnersSummaryCard', () => {
  it('shows a skeleton while the runners load', async () => {
    mount(2, 'hang');
    expect(await screen.findByRole('status', { name: Ar['runners.loading'] })).toBeInTheDocument();
  });

  it('shows an error and loads again on retry', async () => {
    const { runnersGate, user } = mount(2, 'fail');
    expect(await screen.findByText(Ar['runners.error'])).toBeInTheDocument();
    runnersGate.set('pass');
    await user.click(screen.getByRole('button', { name: Ar['jobs.retry'] }));
    expect(await screen.findByRole('list', { name: Ar['home.runners.title'] })).toBeInTheDocument();
  });

  it('offers to add a runner when there is none, if the person may', async () => {
    const onAddRunner = vi.fn();
    const { user } = mount(0, 'pass', onAddRunner);
    expect(await screen.findByText(Ar['home.runners.empty'])).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: Ar['home.runners.add'] }));
    expect(onAddRunner).toHaveBeenCalledOnce();
  });

  it('shows no add button to someone who cannot add runners', async () => {
    mount(0);
    expect(await screen.findByText(Ar['home.runners.empty'])).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: Ar['home.runners.add'] })).not.toBeInTheDocument();
  });

  it('lists five runners with their status and opens one', async () => {
    const { calls, user, onOpenRunner, onShowAll } = mount(7);
    const list = await screen.findByRole('list', { name: Ar['home.runners.title'] });
    expect(within(list).getAllByRole('listitem')).toHaveLength(5);
    expect(within(list).getAllByText(Ar['runners.view.online'])).toHaveLength(3);
    expect(within(list).getAllByText(Ar['runners.view.offline'])).toHaveLength(2);
    expect(calls.map((call) => new URL(call.url).search).filter((search) => search.includes('Limit'))).toEqual(['?Page=1&Limit=5']);
    await user.click(within(list).getByRole('button', { name: /^runner-1/ }));
    expect(onOpenRunner).toHaveBeenCalledWith('0198a000-0000-7000-8000-000000000101');
    await user.click(screen.getByRole('button', { name: Ar['home.runners.all'] }));
    expect(onShowAll).toHaveBeenCalledOnce();
  });
});
