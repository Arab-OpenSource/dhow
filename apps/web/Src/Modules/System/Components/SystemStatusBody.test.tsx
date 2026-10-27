import { screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { apiDown, apiUp, envelope, META_BODY, stubFetch } from '@/Core/Testing/Fetch';
import { renderWithProviders } from '@/Core/Testing/Render';
import { Ar } from '@/I18n/Locales/Ar';
import { SystemStatusBody } from '@/Modules/System/Components/SystemStatusBody';
import { SystemStatusFooter } from '@/Modules/System/Components/SystemStatusFooter';

const open = (onOpenCheck = vi.fn()) =>
  renderWithProviders(
    <>
      <SystemStatusBody onOpenCheck={onOpenCheck} />
      <SystemStatusFooter />
    </>,
  );

const readinessCalls = (calls: Request[]) => calls.filter((call) => call.url.endsWith('/health/ready'));

describe('SystemStatusBody', () => {
  it('shows the version, the commit and the setup state', async () => {
    apiUp();
    open();
    expect(await screen.findByText('v0.1.0')).toBeInTheDocument();
    expect(screen.getByText('2e39188')).toBeInTheDocument();
    expect(screen.getByText(Ar['status.build.setupDone'])).toBeInTheDocument();
  });

  it('isolates version and commit values so they read correctly inside Arabic text', async () => {
    apiUp();
    open();
    const version = await screen.findByText('v0.1.0');
    expect(version.tagName).toBe('BDI');
    expect(version).toHaveAttribute('dir', 'ltr');
    expect(screen.getByText('2e39188').tagName).toBe('BDI');
  });

  it('lists every check with a word and not only a color', async () => {
    apiUp();
    open();
    const database = await screen.findByText(Ar['status.check.database']);
    expect(within(database.closest('li') as HTMLElement).getByText(Ar['status.state.live'])).toBeInTheDocument();
    expect(screen.getByText(Ar['status.check.migrations'])).toBeInTheDocument();
  });

  it('marks a failing check as failed while the server answers 503', async () => {
    const body = {
      Success: false,
      Error: { Code: 'SERVICE_UNAVAILABLE', Message: 'x' },
      Data: {
        Status: 'unavailable',
        Checks: [
          { Name: 'database', Ok: false },
          { Name: 'migrations', Ok: true },
        ],
      },
    };
    stubFetch((request) => (request.url.endsWith('/meta') ? envelope(META_BODY) : envelope(body, 503)));
    open();
    const database = await screen.findByText(Ar['status.check.database']);
    expect(within(database.closest('li') as HTMLElement).getByText(Ar['status.state.failed'])).toBeInTheDocument();
  });

  it('says so when the server reports no checks', async () => {
    apiUp(META_BODY, { Success: true, Data: { Status: 'ok', Checks: [] } });
    open();
    expect(await screen.findByText(Ar['status.checks.empty'])).toBeInTheDocument();
  });

  it('shows a skeleton after a short wait while loading', async () => {
    stubFetch(() => new Promise<Response>(() => undefined));
    open();
    expect(await screen.findByRole('status', { name: Ar['status.loading'] })).toHaveAttribute('aria-busy', 'true');
  });

  it('shows an inline error in the right language and recovers on retry', async () => {
    apiDown();
    const { user } = open();
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent(Ar['status.error.title']);
    expect(alert).toHaveTextContent(Ar['api.error.network']);
    apiUp();
    await user.click(screen.getByRole('button', { name: Ar['status.retry'] }));
    expect(await screen.findByText('v0.1.0')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('shows the server message for a server-side error', async () => {
    stubFetch(() => envelope({ Success: false, Error: { Code: 'INTERNAL', Message: 'حصل خطأ في السيرفر' } }, 500));
    open();
    expect(await screen.findByRole('alert')).toHaveTextContent('حصل خطأ في السيرفر');
  });

  it('refreshes on demand and disables the button while it works', async () => {
    const { calls } = apiUp();
    const { user } = open();
    await screen.findByText('v0.1.0');
    const before = readinessCalls(calls).length;
    await user.click(screen.getByRole('button', { name: Ar['status.refresh'] }));
    await waitFor(() => {
      expect(readinessCalls(calls).length).toBeGreaterThan(before);
    });
    await waitFor(() => {
      expect(screen.getByRole('button', { name: Ar['status.refresh'] })).toBeEnabled();
    });
  });

  it('opens the detail of a check from its row', async () => {
    apiUp();
    const onOpenCheck = vi.fn();
    const { user } = open(onOpenCheck);
    const row = (await screen.findByText(Ar['status.check.migrations'])).closest('button');
    expect(row).not.toBeNull();
    await user.click(row as HTMLElement);
    expect(onOpenCheck).toHaveBeenCalledWith('migrations');
  });
});
