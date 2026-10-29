import { createMemoryHistory } from '@tanstack/react-router';
import { configure, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';

import { App } from '@/App/App';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { useToastStore } from '@/Core/Store/useToastStore';
import { startBackend, type IBackendOptions } from '@/Core/Testing/Backend';
import { gate } from '@/Core/Testing/Gate';
import { findPage } from '@/Core/Testing/Pages';
import { Ar } from '@/I18n/Locales/Ar';

configure({ asyncUtilTimeout: 5000 });

const open = async (options: IBackendOptions = {}) => {
  startBackend(options);
  const router = createAppRouter(createMemoryHistory({ initialEntries: ['/account/security'] }));
  const user = userEvent.setup();
  render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
  const page = await findPage(Ar['shell.nav.security']);
  return { router, user, page };
};

const GROUPS = [Ar['security.group.password'], Ar['security.group.secondFactor'], Ar['security.group.sessions'], Ar['security.group.tokens']];

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
});

describe('the security page', () => {
  it('has the password, second step, sessions and personal tokens sections', async () => {
    const { page } = await open();
    for (const name of GROUPS) expect(await within(page).findByRole('heading', { level: 2, name })).toBeInTheDocument();
    expect(await within(page).findByRole('list', { name: Ar['account.sessions.title'] })).toBeInTheDocument();
    expect(within(page).getByRole('button', { name: Ar['sessions.revokeOthers'] })).toBeInTheDocument();
    expect(within(page).getByRole('button', { name: Ar['account.password.change'] })).toBeInTheDocument();
    expect(await within(page).findByRole('heading', { level: 3, name: Ar['tokens.title'] })).toBeInTheDocument();
  });

  it('shows an amber status line and one primary action until a factor is on', async () => {
    const { page, user, router } = await open();
    const status = (await within(page).findByText(Ar['account.summary.weak.title'])).closest('[data-tone]');
    expect(status).toHaveAttribute('data-tone', 'warn');
    const primary = page.querySelectorAll('[data-variant="primary"]');
    expect(primary).toHaveLength(1);
    expect(primary[0]).toHaveTextContent(Ar['security.enable']);
    await user.click(within(page).getByRole('button', { name: Ar['security.enable'] }));
    expect(await screen.findByRole('dialog', { name: Ar['totp.slide.title'] })).toBeInTheDocument();
    expect(router.state.location.search).toEqual({ slide: 'account-totp' });
  });

  it('turns the status line green and drops the primary action once TOTP is on', async () => {
    const { page } = await open({ mfa: { TotpEnabled: true, RecoveryCodesRemaining: 8, Passkeys: [] } });
    const status = (await within(page).findByText(Ar['account.summary.strong.title'])).closest('[data-tone]');
    expect(status).toHaveAttribute('data-tone', 'ok');
    expect(within(page).queryByRole('button', { name: Ar['security.enable'] })).not.toBeInTheDocument();
    expect(page.querySelector('[data-variant="primary"]')).toBeNull();
  });

  it('shows a skeleton while the second step loads', async () => {
    const hold = gate(/\/me\/mfa$/, 'hang');
    const { page } = await open({ routes: [hold.route] });
    expect(await within(page).findByLabelText(Ar['mfa.section.loading'])).toBeInTheDocument();
    expect(within(page).queryByRole('button', { name: Ar['security.enable'] })).not.toBeInTheDocument();
  });

  it('offers a retry when the second step cannot be loaded', async () => {
    const failing = gate(/\/me\/mfa$/, 'fail');
    const { page } = await open({ routes: [failing.route] });
    expect(await within(page).findByText(Ar['mfa.section.error'])).toBeInTheDocument();
    expect(within(page).getAllByRole('button', { name: Ar['status.retry'] }).length).toBeGreaterThan(0);
  });
});
