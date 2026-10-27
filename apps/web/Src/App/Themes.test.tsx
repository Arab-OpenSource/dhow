import { createMemoryHistory } from '@tanstack/react-router';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { App } from '@/App/App';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';
import { usePaletteStore } from '@/Core/Store/usePaletteStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { apiUp } from '@/Core/Testing/Fetch';
import { readStoredThemes } from '@/Core/Theme/ThemeStorage';
import { DEFAULT_COLORS } from '@/Core/Theme/ThemeTokens';
import { useThemeStore } from '@/Core/Theme/useThemeStore';
import { Ar } from '@/I18n/Locales/Ar';

const html = document.documentElement;

const renderApp = async (url = '/') => {
  const router = createAppRouter(createMemoryHistory({ initialEntries: [url] }));
  const user = userEvent.setup();
  const view = render(<App router={router} queryClient={createQueryClient({ retry: false })} />);
  await screen.findByRole('main', { hidden: true });
  return { router, user, view };
};

const runCommand = async (user: ReturnType<typeof userEvent.setup>, label: string) => {
  await user.keyboard('{Control>}k{/Control}');
  await user.type(await screen.findByRole('combobox'), label);
  await user.keyboard('{Enter}');
};

const findHex = (token: 'canvas' | 'rail') =>
  screen.findByLabelText<HTMLInputElement>(Ar['themeEditor.hexLabel'].replace('{name}', Ar[`theme.token.${token}`]));

beforeEach(() => {
  apiUp();
});

afterEach(() => {
  useToastStore.setState({ Toasts: [] });
  usePaletteStore.setState({ Open: false });
});

describe('opening appearance', () => {
  it('opens from the settings button on the rail', async () => {
    const { router, user } = await renderApp();
    await user.click(screen.getByRole('link', { name: Ar['shell.nav.appearance'] }));
    expect(await screen.findByRole('dialog', { name: Ar['appearance.title'] })).toBeInTheDocument();
    expect(router.state.location.href).toBe('/?stack=appearance');
  });

  it('opens from the command palette', async () => {
    const { user } = await renderApp();
    await runCommand(user, Ar['palette.command.appearance']);
    expect(await screen.findByRole('dialog', { name: Ar['appearance.title'] })).toBeInTheDocument();
  });

  it('opens the theme editor on top of appearance from the command palette', async () => {
    const { router, user } = await renderApp();
    await runCommand(user, Ar['palette.command.createTheme']);
    const dialog = await screen.findByRole('dialog', { name: Ar['themeEditor.defaultName'] });
    expect(within(dialog).getByText('2 من 2')).toBeInTheDocument();
    expect(router.state.location.href).toBe('/?stack=appearance,theme-edit');
    expect(await within(dialog).findByLabelText(Ar['themeEditor.name'])).toBeInTheDocument();
  });

  it('starts a theme from the footer action of appearance', async () => {
    const { router, user } = await renderApp('/?stack=appearance');
    await user.click(await screen.findByRole('button', { name: Ar['appearance.create'] }));
    await screen.findByRole('dialog', { name: Ar['themeEditor.defaultName'] });
    expect(router.state.location.href).toBe('/?stack=appearance,theme-edit');
  });

  it('reopens an existing custom theme from a deep link', async () => {
    useThemeStore.setState({ CustomThemes: [{ Id: 'mine', Name: 'Harbour', Colors: { ...DEFAULT_COLORS } }] });
    await renderApp('/?stack=appearance,theme-edit-mine');
    const dialog = await screen.findByRole('dialog', { name: 'Harbour' });
    expect(await within(dialog).findByLabelText(Ar['themeEditor.name'])).toHaveValue('Harbour');
  });
});

describe('changing and editing themes in the app', () => {
  it('switches theme from appearance and keeps it after a reload', async () => {
    const first = await renderApp('/?stack=appearance');
    await first.user.click(await screen.findByRole('button', { name: Ar['theme.system.indigo'] }));
    expect(html).toHaveAttribute('data-theme', 'indigo');
    first.view.unmount();
    html.removeAttribute('data-theme');
    useThemeStore.setState({ ...readStoredThemes(), Draft: null });
    await renderApp('/?stack=appearance');
    expect(html).toHaveAttribute('data-theme', 'indigo');
    expect(await screen.findByRole('button', { name: Ar['theme.system.indigo'] })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
  });

  it('previews live, and Escape puts the active theme back', async () => {
    const { user } = await renderApp('/?stack=appearance,theme-edit');
    fireEvent.change(await findHex('canvas'), { target: { value: '#123456' } });
    expect(html.style.getPropertyValue('--color-canvas')).toBe('#123456');
    await user.keyboard('{Escape}');
    await screen.findByRole('dialog', { name: Ar['appearance.title'] });
    await waitFor(() => {
      expect(html.style.getPropertyValue('--color-canvas')).toBe('');
    });
    expect(html).toHaveAttribute('data-theme', 'sea');
    expect(useThemeStore.getState().CustomThemes).toEqual([]);
  });

  it('puts the active theme back when every slide is closed at once', async () => {
    const { user } = await renderApp('/?stack=appearance,theme-edit');
    fireEvent.change(await findHex('rail'), { target: { value: '#123456' } });
    expect(html.style.getPropertyValue('--color-rail')).toBe('#123456');
    await user.click(screen.getByRole('button', { name: Ar['slides.closeAll'] }));
    await waitFor(() => {
      expect(html.style.getPropertyValue('--color-rail')).toBe('');
    });
  });

  it('follows the name in the title and the trail, then saves and goes back to appearance', async () => {
    const { router, user } = await renderApp('/?stack=appearance,theme-edit');
    const name = await screen.findByLabelText(Ar['themeEditor.name']);
    await user.clear(name);
    await user.type(name, 'Harbour');
    expect(await screen.findByRole('dialog', { name: 'Harbour' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: Ar['themeEditor.save'] }));
    await screen.findByRole('dialog', { name: Ar['appearance.title'] });
    expect(router.state.location.href).toBe('/?stack=appearance');
    expect(await screen.findByText(Ar['appearance.inUse'])).toBeInTheDocument();
    expect(useThemeStore.getState().CustomThemes.map((theme) => theme.Name)).toEqual(['Harbour']);
  });
});
