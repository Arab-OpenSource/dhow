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

const mount = async (role: string) => {
  startBackend({ role, routes: [orgTokens] });
  const router = createAppRouter(createMemoryHistory({ initialEntries: ['/?stack=organization'] }));
  const user = userEvent.setup();
  render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
  const dialog = await screen.findByRole('dialog', { name: Ar['organization.title'] });
  return { router, user, dialog };
};

describe('reaching the organization API tokens', () => {
  it('opens from the organization footer for an admin', async () => {
    const { dialog, user, router } = await mount('admin');
    await user.click(await within(dialog).findByRole('button', { name: Ar['organization.tokens'] }));
    expect(await screen.findByText(Ar['orgTokens.empty'])).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.search).toEqual({ stack: 'organization,organization-tokens' });
    });
  });

  it('stays out of sight for a developer', async () => {
    const { dialog } = await mount('developer');
    await within(dialog).findByRole('tablist');
    expect(within(dialog).queryByRole('button', { name: Ar['organization.tokens'] })).not.toBeInTheDocument();
  });
});
