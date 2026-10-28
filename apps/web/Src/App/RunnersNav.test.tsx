import { createMemoryHistory } from '@tanstack/react-router';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';

import { parseStack } from '@/App/Panels';
import { App } from '@/App/App';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { apiUp, meBody, META_BODY, READY_BODY } from '@/Core/Testing/Fetch';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Ar } from '@/I18n/Locales/Ar';

const RUNNER_ID = '0198a000-0000-7000-8000-0000000000e1';

const renderApp = async (role: string, url = '/') => {
  apiUp(META_BODY, READY_BODY, meBody(role));
  const router = createAppRouter(createMemoryHistory({ initialEntries: [url] }));
  const user = userEvent.setup();
  render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
  await screen.findByRole('main', { hidden: true }, { timeout: 5000 });
  return { router, user };
};

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
});

describe('the runners section', () => {
  it('accepts the runners slides in the URL and nothing that only looks like them', () => {
    expect(parseStack(`runners,runner-${RUNNER_ID}`)).toEqual(['runners', `runner-${RUNNER_ID}`]);
    expect(parseStack('runners,runner-new')).toEqual(['runners', 'runner-new']);
    expect(parseStack('runners,runner-1')).toBeNull();
  });

  it('opens from the side menu and puts the slide in the URL', async () => {
    const { router, user } = await renderApp('developer');
    const nav = await screen.findByRole('navigation', { name: Ar['shell.nav.label'] });
    await user.click(within(nav).getByRole('link', { name: Ar['shell.nav.runners'] }));
    expect(router.state.location.href).toBe('/?stack=runners');
    expect(await screen.findByRole('dialog', { name: Ar['runners.title'] }, { timeout: 5000 })).toBeInTheDocument();
  });

  it('is not in the side menu for viewers', async () => {
    await renderApp('viewer');
    const nav = await screen.findByRole('navigation', { name: Ar['shell.nav.label'] });
    expect(within(nav).getByRole('link', { name: Ar['shell.nav.team'] })).toBeInTheDocument();
    expect(within(nav).queryByRole('link', { name: Ar['shell.nav.runners'] })).not.toBeInTheDocument();
    expect(within(nav).queryByRole('button', { name: Ar['shell.nav.runners'] })).not.toBeInTheDocument();
  });
});
