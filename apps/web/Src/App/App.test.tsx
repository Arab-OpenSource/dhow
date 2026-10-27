import { createMemoryHistory } from '@tanstack/react-router';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';

import { App } from '@/App/App';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
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

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
  usePaletteStore.setState({ Open: false });
});

describe('app shell', () => {
  it('renders Arabic and right to left by default with one workspace area', async () => {
    await renderApp();
    expect(document.documentElement.dir).toBe('rtl');
    expect(document.documentElement.lang).toBe('ar');
    expect(screen.getAllByRole('main')).toHaveLength(1);
    expect(screen.getByRole('heading', { level: 1, name: Ar['workspace.title'] })).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: Ar['shell.nav.label'] })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: Ar['shell.nav.workspace'] })).toHaveAttribute('aria-current', 'page');
  });

  it('has a skip link that moves focus to the workspace', async () => {
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
    expect(router.state.location.search).toEqual({ panel: 'notifications' });
    expect(router.state.location.href).toBe('/?panel=notifications');
  });
});

describe('slide-over panels in the URL', () => {
  it('opens a deep-linked panel on first render and focuses inside it', async () => {
    await renderApp('/?panel=notifications');
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
    expect(router.state.location.search).toEqual({ panel: 'notifications' });
    await user.keyboard('{Escape}');
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    expect(router.state.location.search).toEqual({});
    expect(bell).toHaveFocus();
  });

  it('keeps an unknown panel closed, says so in a toast and cleans the URL', async () => {
    const { router } = await renderApp('/?panel=does-not-exist');
    expect(await screen.findByText(Ar['toast.unknownPanel.title'])).toBeInTheDocument();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.search).toEqual({});
    });
  });
});

describe('language switch', () => {
  it('flips the whole shell to English and left to right, and remembers it', async () => {
    const { user } = await renderApp();
    await user.click(screen.getByRole('button', { name: Ar['shell.language.switch'].replace('{language}', Ar['language.en']) }));
    expect(document.documentElement.dir).toBe('ltr');
    expect(document.documentElement.lang).toBe('en');
    expect(screen.getByRole('heading', { level: 1, name: En['workspace.title'] })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: En['shell.nav.workspace'] })).toBeInTheDocument();
    expect(window.localStorage.getItem('dhow.lang')).toBe('en');
    await user.click(screen.getByRole('button', { name: En['shell.language.switch'].replace('{language}', En['language.ar']) }));
    expect(document.documentElement.dir).toBe('rtl');
  });
});

describe('unknown routes', () => {
  it('shows a not-found state that leads back to the workspace', async () => {
    const { router, user } = await renderApp('/nowhere');
    expect(await screen.findByText(Ar['notFound.text'])).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: Ar['notFound.action'] }));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/');
    });
  });
});
