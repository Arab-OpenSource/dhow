import { createMemoryHistory } from '@tanstack/react-router';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { App } from '@/App/App';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { apiDown, apiUp, META_BODY } from '@/Core/Testing/Fetch';
import { useToastStore } from '@/Core/Store/useToastStore';
import { usePaletteStore } from '@/Core/Store/usePaletteStore';
import { En } from '@/I18n/Locales/En';
import { Ar } from '@/I18n/Locales/Ar';

const renderApp = async (url = '/') => {
  const router = createAppRouter(createMemoryHistory({ initialEntries: [url] }));
  const user = userEvent.setup();
  render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
  await screen.findByRole('main', { hidden: true });
  return { router, user };
};

beforeEach(() => {
  apiUp();
});

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
  usePaletteStore.setState({ Open: false });
});

describe('app shell', () => {
  it('renders Arabic and right to left by default with one main area', async () => {
    await renderApp();
    expect(document.documentElement.dir).toBe('rtl');
    expect(document.documentElement.lang).toBe('ar');
    expect(screen.getAllByRole('main')).toHaveLength(1);
    expect(screen.getByRole('heading', { level: 1, name: Ar['home.title'] })).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: Ar['shell.nav.label'] })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: Ar['shell.nav.home'] })).toHaveAttribute('aria-current', 'page');
  });

  it('has a skip link that moves focus to the main area', async () => {
    const { user } = await renderApp();
    await user.click(screen.getByText(Ar['shell.skipToContent']));
    expect(screen.getByRole('main')).toHaveFocus();
  });

  it('opens the command palette from the search button and from Ctrl+K', async () => {
    const { user } = await renderApp();
    await user.click(screen.getByRole('button', { name: Ar['shell.search'] }));
    expect(await screen.findByRole('combobox', { name: Ar['palette.label'] })).toBeInTheDocument();
    await user.keyboard('{Escape}');
    await waitFor(() => {
      expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    });
    await user.keyboard('{Control>}k{/Control}');
    expect(await screen.findByRole('combobox', { name: Ar['palette.label'] })).toHaveFocus();
  });

  it('runs a palette command and puts the panel in the URL', async () => {
    const { router, user } = await renderApp();
    await user.keyboard('{Control>}k{/Control}');
    await user.type(await screen.findByRole('combobox'), Ar['notifications.title']);
    await user.keyboard('{Enter}');
    expect(await screen.findByRole('dialog', { name: Ar['notifications.title'] })).toBeInTheDocument();
    expect(router.state.location.href).toBe('/?stack=notifications');
  });
});

describe('slides in the URL', () => {
  it('opens a deep-linked panel on first render and focuses inside it', async () => {
    await renderApp('/?stack=notifications');
    const dialog = await screen.findByRole('dialog', { name: Ar['notifications.title'] });
    expect(within(dialog).getByText(Ar['notifications.empty'])).toBeInTheDocument();
    await waitFor(() => {
      expect(dialog).toContainElement(document.activeElement as HTMLElement);
    });
  });

  it('closes with Escape, drops the param and returns focus to the opener', async () => {
    const { router, user } = await renderApp();
    const bell = screen.getByRole('button', { name: Ar['shell.notifications'] });
    await user.click(bell);
    await screen.findByRole('dialog');
    expect(router.state.location.href).toBe('/?stack=notifications');
    await user.keyboard('{Escape}');
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    expect(router.state.location.href).toBe('/');
    await waitFor(() => {
      expect(bell).toHaveFocus();
    });
  });

  it('keeps an unknown slide closed, says so in a toast and cleans the URL', async () => {
    const { router } = await renderApp('/?stack=does-not-exist');
    expect(await screen.findByText(Ar['toast.unknownPanel.title'])).toBeInTheDocument();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.href).toBe('/');
    });
  });
});

describe('language switch', () => {
  it('flips the whole shell to English and left to right, and remembers it', async () => {
    const { user } = await renderApp();
    await user.click(screen.getByRole('button', { name: Ar['shell.language.switch'].replace('{language}', Ar['language.en']) }));
    expect(document.documentElement.dir).toBe('ltr');
    expect(document.documentElement.lang).toBe('en');
    expect(screen.getByRole('heading', { level: 1, name: En['home.title'] })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: En['shell.nav.home'] })).toBeInTheDocument();
    expect(window.localStorage.getItem('dhow.lang')).toBe('en');
    await user.click(screen.getByRole('button', { name: En['shell.language.switch'].replace('{language}', En['language.ar']) }));
    expect(document.documentElement.dir).toBe('rtl');
  });
});

describe('unknown routes', () => {
  it('shows a not-found state that leads back to Home', async () => {
    const { router, user } = await renderApp('/nowhere');
    expect(await screen.findByText(Ar['notFound.text'])).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: Ar['notFound.action'] }));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/');
    });
  });
});

describe('system module in the shell', () => {
  it('opens server status from the Home list and shows the build', async () => {
    const { router, user } = await renderApp();
    await user.click(screen.getByRole('button', { name: new RegExp(Ar['status.title']) }));
    const dialog = await screen.findByRole('dialog', { name: Ar['status.title'] });
    expect(await within(dialog).findByText('v0.1.0')).toBeInTheDocument();
    expect(router.state.location.href).toBe('/?stack=system-status');
  });

  it('opens server status from a deep link', async () => {
    await renderApp('/?stack=system-status');
    const dialog = await screen.findByRole('dialog', { name: Ar['status.title'] });
    expect(await within(dialog).findByText('2e39188')).toBeInTheDocument();
  });

  it('shows the first-time setup banner when the server says setup is pending', async () => {
    apiUp({ ...META_BODY, Data: { ...META_BODY.Data, SetupRequired: true } });
    await renderApp();
    expect(await screen.findByText(Ar['setup.banner.title'])).toBeInTheDocument();
  });

  it('keeps the shell working with an error banner while the API is down', async () => {
    apiDown();
    const { user } = await renderApp();
    expect(await screen.findByRole('alert')).toHaveTextContent(Ar['apiDown.title']);
    await user.click(screen.getByRole('button', { name: Ar['shell.search'] }));
    expect(await screen.findByRole('combobox')).toBeInTheDocument();
    await user.keyboard('{Escape}');
    await waitFor(() => {
      expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    });
    await user.click(screen.getByRole('button', { name: Ar['shell.notifications'] }));
    expect(await screen.findByRole('dialog', { name: Ar['notifications.title'] })).toBeInTheDocument();
  });
});
