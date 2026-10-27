import { afterEach, describe, expect, it, vi } from 'vitest';

import { readStoredThemes, ACTIVE_THEME_KEY, CUSTOM_THEMES_KEY } from '@/Core/Theme/ThemeStorage';
import { DEFAULT_COLORS, THEME_TOKEN_KEYS } from '@/Core/Theme/ThemeTokens';
import {
  cleanThemeName,
  isCustomThemeId,
  isHex,
  sanitizeColors,
  sanitizeCustomTheme,
  sanitizeCustomThemes,
  THEME_NAME_MAX,
} from '@/Core/Theme/ThemeValidation';

const colors = (overrides: Record<string, unknown> = {}) => ({ ...DEFAULT_COLORS, ...overrides });

afterEach(() => {
  window.localStorage.clear();
});

describe('hex validation', () => {
  it('accepts only #rrggbb', () => {
    expect(isHex('#a1B2c3')).toBe(true);
    expect(isHex('#000000')).toBe(true);
  });

  it.each([
    'red',
    '#fff',
    'url(javascript:alert(1))',
    '#12345g',
    '#1234567',
    '#12345678',
    ' #123456',
    '#123456 ',
    '#123456;color:red',
    'rgb(0,0,0)',
    'var(--color-canvas)',
    '',
  ])('rejects %j', (value) => {
    expect(isHex(value)).toBe(false);
  });

  it('rejects values that are not strings', () => {
    for (const value of [null, undefined, 123456, {}, ['#123456']]) expect(isHex(value)).toBe(false);
  });
});

describe('custom theme identity', () => {
  it('allows lowercase letters, digits and dashes only', () => {
    expect(isCustomThemeId('my-theme-2')).toBe(true);
    for (const id of ['', 'My', 'a b', 'a_b', '<x>', 'a'.repeat(41), 'ا', '__proto__']) {
      expect(isCustomThemeId(id)).toBe(false);
    }
  });

  it('never lets a custom theme take a system theme id', () => {
    for (const id of ['sea', 'teak', 'dusk', 'indigo']) expect(isCustomThemeId(id)).toBe(false);
  });

  it('trims names, strips control characters and clamps them to 40 characters', () => {
    expect(cleanThemeName('  بحري\u0007  ')).toBe('بحري');
    expect(cleanThemeName('x'.repeat(100))).toHaveLength(THEME_NAME_MAX);
    expect(cleanThemeName('   ')).toBeNull();
    expect(cleanThemeName(42)).toBeNull();
  });
});

describe('colour sets', () => {
  it('keeps known tokens with valid hex, lowercased, and falls back for everything else', () => {
    const result = sanitizeColors({
      canvas: '#ABCDEF',
      rail: 'red',
      input: 'url(https://evil.example/x)',
      surface: '#fff',
      unknown: '#123456',
      '--color-canvas': '#123456',
      __proto__: { raised: '#123456' },
    });
    expect(result.canvas).toBe('#abcdef');
    expect(result.rail).toBe(DEFAULT_COLORS.rail);
    expect(result.input).toBe(DEFAULT_COLORS.input);
    expect(result.surface).toBe(DEFAULT_COLORS.surface);
    expect(result.raised).toBe(DEFAULT_COLORS.raised);
    expect(Object.keys(result).sort()).toEqual([...THEME_TOKEN_KEYS].sort());
  });

  it('returns the fallback for input that is not an object', () => {
    for (const input of [null, 'x', 12, [], undefined]) expect(sanitizeColors(input)).toEqual(DEFAULT_COLORS);
  });
});

describe('custom themes from storage', () => {
  it('drops entries with a bad id, a missing name or no colour object', () => {
    const list = sanitizeCustomThemes([
      { Id: 'ok', Name: 'Good', Colors: colors() },
      { Id: 'Bad Id', Name: 'x', Colors: colors() },
      { Id: 'sea', Name: 'x', Colors: colors() },
      { Id: 'noname', Name: '', Colors: colors() },
      { Id: 'nocolors', Name: 'x', Colors: 'red' },
      { Id: 5, Name: 'x', Colors: colors() },
      null,
      'string',
      42,
    ]);
    expect(list.map((theme) => theme.Id)).toEqual(['ok']);
  });

  it('removes duplicate ids, clamps names and cleans every colour', () => {
    const [first, ...rest] = sanitizeCustomThemes([
      { Id: 'dup', Name: 'n'.repeat(90), Colors: colors({ canvas: 'javascript:alert(1)', a1: '#FFAA00' }) },
      { Id: 'dup', Name: 'second', Colors: colors() },
    ]);
    expect(rest).toEqual([]);
    expect(first?.Name).toHaveLength(THEME_NAME_MAX);
    expect(first?.Colors.canvas).toBe(DEFAULT_COLORS.canvas);
    expect(first?.Colors.a1).toBe('#ffaa00');
  });

  it('returns nothing for a payload that is not an array', () => {
    for (const input of [{}, 'x', null, 7, { length: 3 }]) expect(sanitizeCustomThemes(input)).toEqual([]);
  });

  it('rejects a single hostile theme object', () => {
    expect(sanitizeCustomTheme({ Id: '"><img src=x onerror=alert(1)>', Name: 'x', Colors: colors() })).toBeNull();
  });
});

describe('storage', () => {
  it('sanitizes a malicious localStorage payload on load', () => {
    window.localStorage.setItem(
      CUSTOM_THEMES_KEY,
      JSON.stringify([
        { Id: 'mine', Name: 'Mine', Colors: colors({ canvas: 'url(javascript:alert(1))', rail: '#112233' }) },
        { Id: '</style><script>alert(1)</script>', Name: 'x', Colors: colors() },
      ]),
    );
    window.localStorage.setItem(ACTIVE_THEME_KEY, 'mine');
    const stored = readStoredThemes();
    expect(stored.ActiveId).toBe('mine');
    expect(stored.CustomThemes).toHaveLength(1);
    expect(stored.CustomThemes[0]?.Colors.canvas).toBe(DEFAULT_COLORS.canvas);
    expect(stored.CustomThemes[0]?.Colors.rail).toBe('#112233');
  });

  it('falls back to the default theme for an unknown or missing active id', () => {
    window.localStorage.setItem(ACTIVE_THEME_KEY, 'ghost');
    expect(readStoredThemes().ActiveId).toBe('sea');
    window.localStorage.setItem(ACTIVE_THEME_KEY, '<script>');
    expect(readStoredThemes().ActiveId).toBe('sea');
  });

  it('survives broken JSON and a localStorage that throws', () => {
    window.localStorage.setItem(CUSTOM_THEMES_KEY, '{not json');
    expect(readStoredThemes()).toEqual({ ActiveId: 'sea', CustomThemes: [] });
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked');
    });
    expect(readStoredThemes()).toEqual({ ActiveId: 'sea', CustomThemes: [] });
  });
});
