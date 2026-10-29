import { screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { envelope, META_BODY } from '@/Core/Testing/Fetch';
import { startBackend } from '@/Core/Testing/Backend';
import { gate, type TGateMode } from '@/Core/Testing/Gate';
import { renderWithProviders } from '@/Core/Testing/Render';
import { Ar } from '@/I18n/Locales/Ar';
import { ServerStatusCard } from '@/Modules/System/Components/ServerStatusCard';

const mount = (mode: TGateMode = 'pass', checks?: { Name: string; Ok: boolean }[]) => {
  const readyGate = gate(/\/health\/ready$/, mode);
  const backend = startBackend({
    routes: [
      readyGate.route,
      (_request, url) =>
        checks !== undefined && url.pathname.endsWith('/health/ready')
          ? envelope({ Success: true, Data: { Status: checks.every((check) => check.Ok) ? 'ok' : 'unavailable', Checks: checks } })
          : undefined,
    ],
  });
  const onOpenCheck = vi.fn();
  const onOpenStatus = vi.fn();
  const view = renderWithProviders(<ServerStatusCard onOpenCheck={onOpenCheck} onOpenStatus={onOpenStatus} />);
  return { ...view, ...backend, readyGate, onOpenCheck, onOpenStatus };
};

describe('ServerStatusCard', () => {
  it('shows a skeleton while the checks load', async () => {
    mount('hang');
    expect(await screen.findByRole('status', { name: Ar['status.loading'] })).toBeInTheDocument();
  });

  it('shows an error and loads again on retry', async () => {
    const { readyGate, user } = mount('fail');
    expect(await screen.findByText(Ar['home.status.error'])).toBeInTheDocument();
    readyGate.set('pass');
    await user.click(screen.getByRole('button', { name: Ar['jobs.retry'] }));
    expect(await screen.findByText(Ar['status.summary.ok.title'])).toBeInTheDocument();
  });

  it('says so when the server reports no checks', async () => {
    mount('pass', []);
    expect(await screen.findByText(Ar['home.status.empty'])).toBeInTheDocument();
  });

  it('reuses the status summary, lists the checks and the version, and opens them', async () => {
    const { user, onOpenCheck, onOpenStatus } = mount('pass', [
      { Name: 'database', Ok: true },
      { Name: 'migrations', Ok: false },
    ]);
    expect(await screen.findByText(Ar['status.summary.bad.title'])).toBeInTheDocument();
    const list = screen.getByRole('list', { name: Ar['status.checks.title'] });
    expect(within(list).getByText(META_BODY.Data.Version)).toBeInTheDocument();
    await user.click(within(list).getByRole('button', { name: new RegExp(Ar['status.check.migrations']) }));
    expect(onOpenCheck).toHaveBeenCalledWith('migrations');
    await user.click(screen.getByRole('button', { name: Ar['home.status.open'] }));
    expect(onOpenStatus).toHaveBeenCalledOnce();
  });
});
