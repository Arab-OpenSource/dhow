import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { runInNewContext } from 'node:vm';

import { describe, expect, it, vi } from 'vitest';

import { applyColors, applyTheme } from '@/Core/Theme/ApplyTheme';
import { ACTIVE_THEME_KEY, CUSTOM_THEMES_KEY, readStoredThemes } from '@/Core/Theme/ThemeStorage';
import { DEFAULT_COLORS, SYSTEM_THEMES, THEME_TOKEN_KEYS, THEME_VARIABLES } from '@/Core/Theme/ThemeTokens';
import { useThemeStore } from '@/Core/Theme/useThemeStore';

const root = () => document.documentElement;
const variable = (key: (typeof THEME_TOKEN_KEYS)[number]) => root().style.getPropertyValue(THEME_VARIABLES[key]);
const teak = SYSTEM_THEMES.find((theme) => theme.Id === 'teak');
const store = () => useThemeStore.getState();

describe('applying a theme', () => {
  it('sets data-theme for a system theme and writes no inline colours', () => {
    applyTheme('teak', []);
    expect(root().getAttribute('data-theme')).toBe('teak');
    expect(root().getAttribute('style') ?? '').toBe('');
  });

  it('sets the variables with the CSSOM for a custom theme and drops data-theme', () => {
    const before = document.querySelectorAll('style').length;
    const colors = { ...DEFAULT_COLORS, canvas: '#123456' };
    applyTheme('mine', [{ Id: 'mine', Name: 'Mine', Colors: colors }]);
    expect(root().hasAttribute('data-theme')).toBe(false);
    expect(variable('canvas')).toBe('#123456');
    expect(variable('a1')).toBe(DEFAULT_COLORS.a1);
    expect(document.querySelectorAll('style')).toHaveLength(before);
  });

  it('clears the custom variables when a system theme is applied afterwards', () => {
    applyTheme('mine', [{ Id: 'mine', Name: 'Mine', Colors: { ...DEFAULT_COLORS, canvas: '#123456' } }]);
    applyTheme('dusk', []);
    expect(variable('canvas')).toBe('');
    expect(root().getAttribute('data-theme')).toBe('dusk');
  });

  it('falls back to the default theme for an unknown id', () => {
    expect(applyTheme('ghost', [])).toBe('sea');
    expect(root().getAttribute('data-theme')).toBe('sea');
  });

  it('refuses to write a value that is not a hex colour', () => {
    applyColors({ ...DEFAULT_COLORS, canvas: 'url(https://evil.example/x)', rail: 'red' });
    expect(variable('canvas')).toBe('');
    expect(variable('rail')).toBe('');
    expect(variable('input')).toBe(DEFAULT_COLORS.input);
  });

  it('keeps the browser colour in step with the canvas', () => {
    const meta = document.createElement('meta');
    meta.name = 'theme-color';
    document.head.append(meta);
    applyTheme('teak', []);
    expect(meta.content).toBe(teak?.Colors.canvas);
    meta.remove();
  });
});

describe('activating a theme', () => {
  it('persists the choice and applies it', () => {
    store().Activate('indigo');
    expect(store().ActiveId).toBe('indigo');
    expect(window.localStorage.getItem(ACTIVE_THEME_KEY)).toBe('indigo');
    expect(root().getAttribute('data-theme')).toBe('indigo');
    expect(readStoredThemes().ActiveId).toBe('indigo');
  });

  it('ignores an id that does not exist', () => {
    store().Activate('ghost');
    expect(store().ActiveId).toBe('sea');
  });
});

describe('editing a draft', () => {
  const begin = (SourceId?: string) => {
    store().BeginDraft({ SourceId, DefaultName: 'My theme' });
  };

  it('previews colours live and restores the active theme when cancelled', () => {
    store().Activate('teak');
    begin();
    store().SetDraftColor('canvas', '#010203');
    expect(variable('canvas')).toBe('#010203');
    store().DiscardDraft();
    expect(store().Draft).toBeNull();
    expect(variable('canvas')).toBe('');
    expect(root().getAttribute('data-theme')).toBe('teak');
  });

  it('starts a new draft from the active theme', () => {
    store().Activate('dusk');
    begin();
    expect(store().Draft?.Colors).toEqual(SYSTEM_THEMES.find((theme) => theme.Id === 'dusk')?.Colors);
    expect(store().Draft?.Existing).toBe(false);
    expect(store().Draft?.Name).toBe('My theme');
  });

  it('rejects a value that is not a hex colour', () => {
    begin();
    store().SetDraftColor('canvas', 'red');
    store().SetDraftColor('rail', '#12345g');
    expect(store().Draft?.Colors.canvas).toBe(DEFAULT_COLORS.canvas);
    expect(store().Draft?.Colors.rail).toBe(DEFAULT_COLORS.rail);
  });

  it('saves a new theme, persists it, activates it and ends the draft', () => {
    begin();
    store().SetDraftName('  Harbour  ');
    store().SetDraftColors({ canvas: '#0A0B0C', a1: '#ff8800' });
    const saved = store().SaveDraft();
    expect(saved?.Name).toBe('Harbour');
    expect(store().Draft).toBeNull();
    expect(store().ActiveId).toBe(saved?.Id);
    expect(variable('canvas')).toBe('#0a0b0c');
    expect(root().hasAttribute('data-theme')).toBe(false);
    const stored = readStoredThemes();
    expect(stored.ActiveId).toBe(saved?.Id);
    expect(stored.CustomThemes).toHaveLength(1);
    expect(stored.CustomThemes[0]?.Colors.a1).toBe('#ff8800');
    expect(JSON.parse(window.localStorage.getItem(CUSTOM_THEMES_KEY) ?? '[]')).toHaveLength(1);
  });

  it('refuses to save a draft without a name', () => {
    begin();
    store().SetDraftName('   ');
    expect(store().SaveDraft()).toBeNull();
    expect(store().CustomThemes).toEqual([]);
  });

  it('edits an existing custom theme in place', () => {
    begin();
    const first = store().SaveDraft();
    begin(first?.Id);
    expect(store().Draft).toMatchObject({ Id: first?.Id, Existing: true });
    store().SetDraftName('Renamed');
    store().SaveDraft();
    expect(store().CustomThemes).toHaveLength(1);
    expect(store().CustomThemes[0]?.Name).toBe('Renamed');
  });

  it('deletes a custom theme and falls back to the default when it was active', () => {
    begin();
    const saved = store().SaveDraft();
    store().RemoveTheme(saved?.Id ?? '');
    expect(store().CustomThemes).toEqual([]);
    expect(store().ActiveId).toBe('sea');
    expect(root().getAttribute('data-theme')).toBe('sea');
    expect(readStoredThemes().CustomThemes).toEqual([]);
  });

  it('keeps working when localStorage refuses writes', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota');
    });
    begin();
    expect(store().SaveDraft()).not.toBeNull();
    store().Activate('teak');
    expect(store().ActiveId).toBe('teak');
  });
});

const initSource = readFileSync(join(process.cwd(), 'public', 'ThemeInit.js'), 'utf8');
const runInit = () => {
  runInNewContext(initSource, { document, window });
};

describe('ThemeInit.js before first paint', () => {
  it('applies a saved system theme', () => {
    window.localStorage.setItem(ACTIVE_THEME_KEY, 'indigo');
    runInit();
    expect(root().getAttribute('data-theme')).toBe('indigo');
  });

  it('applies a saved custom theme with the same variables the app uses', () => {
    store().BeginDraft({ DefaultName: 'x' });
    store().SetDraftColors({ canvas: '#102030' });
    store().SaveDraft();
    const expected = THEME_TOKEN_KEYS.map((key) => variable(key));
    root().removeAttribute('style');
    root().removeAttribute('data-theme');
    runInit();
    expect(THEME_TOKEN_KEYS.map((key) => variable(key))).toEqual(expected);
  });

  it('does nothing for a missing or unknown theme', () => {
    runInit();
    expect(root().getAttribute('style') ?? '').toBe('');
    window.localStorage.setItem(ACTIVE_THEME_KEY, '<img src=x>');
    runInit();
    expect(root().getAttribute('style') ?? '').toBe('');
    expect(root().hasAttribute('data-theme')).toBe(false);
  });

  it('never writes a hostile value from storage', () => {
    window.localStorage.setItem(ACTIVE_THEME_KEY, 'evil');
    window.localStorage.setItem(
      CUSTOM_THEMES_KEY,
      JSON.stringify([
        {
          Id: 'evil',
          Name: 'x',
          Colors: { canvas: 'url(javascript:alert(1))', rail: 'red', input: '#fff', surface: '#112233', 'bad-key': '#123456' },
        },
      ]),
    );
    runInit();
    expect(variable('canvas')).toBe('');
    expect(variable('rail')).toBe('');
    expect(variable('input')).toBe('');
    expect(variable('surface')).toBe('#112233');
    expect(root().getAttribute('style')).not.toContain('bad-key');
    expect(root().getAttribute('style')).not.toContain('javascript');
  });

  it('survives broken JSON and a localStorage that throws', () => {
    window.localStorage.setItem(ACTIVE_THEME_KEY, 'mine');
    window.localStorage.setItem(CUSTOM_THEMES_KEY, '{nope');
    expect(runInit).not.toThrow();
    expect(root().getAttribute('data-theme')).toBe('sea');
  });
});
