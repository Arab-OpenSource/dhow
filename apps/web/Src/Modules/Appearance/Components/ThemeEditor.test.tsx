import { fireEvent, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { useToastStore } from '@/Core/Store/useToastStore';
import { renderWithProviders } from '@/Core/Testing/Render';
import { deriveBackgrounds } from '@/Core/Theme/ThemeColor';
import { CUSTOM_THEMES_KEY } from '@/Core/Theme/ThemeStorage';
import { DEFAULT_COLORS, SYSTEM_THEMES, type TThemeToken } from '@/Core/Theme/ThemeTokens';
import { useThemeStore } from '@/Core/Theme/useThemeStore';
import { Ar } from '@/I18n/Locales/Ar';
import { ThemeEditor } from '@/Modules/Appearance/Components/ThemeEditor';
import { ThemeEditorFooter } from '@/Modules/Appearance/Components/ThemeEditorFooter';

const html = document.documentElement;
const variable = (name: string) => html.style.getPropertyValue(`--color-${name}`);
const hexOf = (token: TThemeToken) =>
  screen.getByLabelText<HTMLInputElement>(Ar['themeEditor.hexLabel'].replace('{name}', Ar[`theme.token.${token}`]));
const pickerOf = (token: TThemeToken) => screen.getByLabelText<HTMLInputElement>(Ar[`theme.token.${token}`]);
const type = (input: HTMLInputElement, value: string) => {
  fireEvent.change(input, { target: { value } });
};

const mount = async (sourceId?: string) => {
  const onDone = vi.fn();
  const onCancel = vi.fn();
  const view = renderWithProviders(
    <>
      <ThemeEditor sourceId={sourceId} onDone={onDone} />
      <ThemeEditorFooter onCancel={onCancel} onDone={onDone} />
    </>,
  );
  await screen.findByLabelText(Ar['themeEditor.name']);
  return { ...view, onDone, onCancel };
};

describe('ThemeEditor form', () => {
  it('starts from the active theme with a default name, five groups and 25 colour rows', async () => {
    await mount();
    expect(screen.getByLabelText(Ar['themeEditor.name'])).toHaveValue(Ar['themeEditor.defaultName']);
    for (const group of ['bg', 'text', 'aurora', 'edge', 'status'] as const) {
      expect(screen.getByRole('heading', { name: Ar[`theme.group.${group}`] })).toBeInTheDocument();
    }
    expect(document.querySelectorAll('input[type="color"]')).toHaveLength(26);
    expect(document.querySelectorAll('input[type="text"][dir="ltr"]')).toHaveLength(25);
    expect(hexOf('canvas')).toHaveValue(DEFAULT_COLORS.canvas);
  });

  it('shows a colour change on the whole app as it happens', async () => {
    await mount();
    type(hexOf('canvas'), '#123456');
    expect(variable('canvas')).toBe('#123456');
    expect(pickerOf('canvas')).toHaveValue('#123456');
    type(pickerOf('rail'), '#abcdef');
    expect(variable('rail')).toBe('#abcdef');
    expect(hexOf('rail')).toHaveValue('#abcdef');
  });

  it('marks a bad hex as invalid and does not apply it', async () => {
    await mount();
    for (const bad of ['red', '#fff', '#12345g', 'url(x)']) {
      type(hexOf('canvas'), bad);
      expect(hexOf('canvas')).toHaveAttribute('aria-invalid', 'true');
      expect(variable('canvas')).toBe(DEFAULT_COLORS.canvas);
    }
    type(hexOf('canvas'), '#112233');
    expect(hexOf('canvas')).not.toHaveAttribute('aria-invalid');
    expect(variable('canvas')).toBe('#112233');
  });

  it('starts from a system theme', async () => {
    const { user } = await mount();
    await user.click(screen.getByRole('button', { name: Ar['theme.system.teak'] }));
    const teak = SYSTEM_THEMES.find((theme) => theme.Id === 'teak');
    expect(variable('canvas')).toBe(teak?.Colors.canvas);
    expect(hexOf('a1')).toHaveValue(teak?.Colors.a1);
  });

  it('generates every background from one colour', async () => {
    await mount();
    type(screen.getByLabelText(Ar['themeEditor.generate.label']), '#336699');
    const derived = deriveBackgrounds('#336699');
    expect(variable('canvas')).toBe(derived.canvas);
    expect(variable('track')).toBe(derived.track);
    expect(hexOf('surface')).toHaveValue(derived.surface);
  });
});

describe('ThemeEditor feedback and saving', () => {
  it('shows the three contrast pairs and flags one under 4.5 to 1', async () => {
    await mount();
    const list = screen.getByRole('list', { name: Ar['themeEditor.contrast'] });
    expect(list.querySelectorAll('li[data-ok="true"]')).toHaveLength(3);
    type(hexOf('t1'), DEFAULT_COLORS.surface);
    const failing = list.querySelector('li[data-ok="false"]');
    expect(failing).toHaveTextContent(Ar['themeEditor.contrast.textOnCards']);
    expect(failing).toHaveTextContent('1.0:1');
  });

  it('warns when two status colours are too alike', async () => {
    await mount();
    type(hexOf('blue'), DEFAULT_COLORS.green);
    const text = Ar['themeEditor.similar']
      .replace('{first}', Ar['theme.token.green'])
      .replace('{second}', Ar['theme.token.blue']);
    expect(screen.getByText(text)).toBeInTheDocument();
  });

  it('will not save without a name', async () => {
    const { user, onDone } = await mount();
    await user.clear(screen.getByLabelText(Ar['themeEditor.name']));
    await user.click(screen.getByRole('button', { name: Ar['themeEditor.save'] }));
    expect(await screen.findByRole('alert')).toHaveTextContent(Ar['themeEditor.name.required']);
    expect(screen.getByLabelText(Ar['themeEditor.name'])).toHaveAttribute('aria-invalid', 'true');
    expect(onDone).not.toHaveBeenCalled();
    expect(useThemeStore.getState().CustomThemes).toEqual([]);
  });

  it('saves, persists and applies the theme', async () => {
    const { user, onDone } = await mount();
    await user.clear(screen.getByLabelText(Ar['themeEditor.name']));
    await user.type(screen.getByLabelText(Ar['themeEditor.name']), 'Harbour');
    type(hexOf('canvas'), '#0a0b0c');
    await user.click(screen.getByRole('button', { name: Ar['themeEditor.save'] }));
    await waitFor(() => {
      expect(onDone).toHaveBeenCalled();
    });
    const [saved] = useThemeStore.getState().CustomThemes;
    expect(saved?.Name).toBe('Harbour');
    expect(useThemeStore.getState().ActiveId).toBe(saved?.Id);
    expect(variable('canvas')).toBe('#0a0b0c');
    expect(JSON.parse(window.localStorage.getItem(CUSTOM_THEMES_KEY) ?? '[]')).toHaveLength(1);
    expect(useToastStore.getState().Toasts.map((toast) => toast.Title)).toContain(
      Ar['themeEditor.saved'].replace('{name}', 'Harbour'),
    );
  });

  it('edits and deletes an existing theme', async () => {
    useThemeStore.setState({
      CustomThemes: [{ Id: 'mine', Name: 'Harbour', Colors: { ...DEFAULT_COLORS } }],
      ActiveId: 'mine',
    });
    const { user, onDone } = await mount('mine');
    expect(screen.getByLabelText(Ar['themeEditor.name'])).toHaveValue('Harbour');
    await user.click(screen.getByRole('button', { name: Ar['themeEditor.delete'] }));
    expect(useThemeStore.getState().CustomThemes).toEqual([]);
    expect(useThemeStore.getState().ActiveId).toBe('sea');
    expect(onDone).toHaveBeenCalled();
    expect(html).toHaveAttribute('data-theme', 'sea');
  });

  it('offers no delete for a new theme and cancels through the callback', async () => {
    const { user, onCancel } = await mount();
    expect(screen.queryByRole('button', { name: Ar['themeEditor.delete'] })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: Ar['themeEditor.cancel'] }));
    expect(onCancel).toHaveBeenCalled();
  });
});
