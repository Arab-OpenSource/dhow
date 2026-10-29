import { createMemoryHistory } from '@tanstack/react-router';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { App } from '@/App/App';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { startBackend } from '@/Core/Testing/Backend';
import { envelope } from '@/Core/Testing/Fetch';
import { Ar } from '@/I18n/Locales/Ar';

const orgTokens = (_request: Request, url: URL) =>
  /\/organizations\/[^/]+\/api-tokens$/.test(url.pathname) ? envelope({ Success: true, Data: [] }) : undefined;

const mount = async (role: string, url = '/org/settings') => {
  startBackend({ role, routes: [orgTokens] });
  const router = createAppRouter(createMemoryHistory({ initialEntries: [url] }));
  const user = userEvent.setup();
  render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
  const nav = await screen.findByRole('navigation', { name: Ar['shell.nav.label'] }, { timeout: 5000 });
  return { router, user, nav };
};

describe('reaching the organization API keys', () => {
  it('opens from the side menu for an admin, with a new-key slide over the page', async () => {
    const { nav, user, router } = await mount('admin', '/');
    await user.click(within(nav).getByRole('link', { name: Ar['shell.nav.apiKeys'] }));
    expect(await screen.findByText(Ar['orgTokens.empty'], {}, { timeout: 5000 })).toBeInTheDocument();
    expect(router.state.location.href).toBe('/org/api-keys');
    const [action] = screen.getAllByRole('button', { name: Ar['tokens.new'] });
    await user.click(action ?? document.body);
    expect(await screen.findByRole('dialog', { name: Ar['tokens.slide.title'] }, { timeout: 5000 })).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.href).toBe('/org/api-keys?slide=organization-token-new');
    });
  });

  it('opens from the organization settings for an admin', async () => {
    const { user, router } = await mount('admin');
    await user.click(await screen.findByRole('button', { name: Ar['organization.tokens.open'] }, { timeout: 5000 }));
    await waitFor(() => {
      expect(router.state.location.href).toBe('/org/api-keys');
    });
  });

  it('stays out of sight for a developer', async () => {
    const { nav } = await mount('developer');
    await screen.findByRole('heading', { level: 1, name: Ar['shell.nav.orgSettings'] });
    expect(within(nav).queryByRole('link', { name: Ar['shell.nav.apiKeys'] })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: Ar['organization.tokens.open'] })).not.toBeInTheDocument();
  });
});
