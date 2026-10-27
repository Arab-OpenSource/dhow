import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import { describe, expect, it } from 'vitest';

interface IColorToken {
  name: string;
  value: { dark: string };
}

interface ITokenFile {
  color: { tokens: IColorToken[] };
}

interface IThemeFile {
  default: string;
  tokens: { key: string; variable: string }[];
  themes: { id: string; colors: Record<string, string> }[];
}

const read = (name: string): string => readFileSync(fileURLToPath(new URL(name, import.meta.url)), 'utf8');

const tokens = JSON.parse(read('./Tokens.json')) as ITokenFile;
const themes = JSON.parse(read('./Themes.json')) as IThemeFile;
const generated = read('./Tokens.css');
const sources = import.meta.glob<string>('../**/*.{ts,tsx}', { query: '?raw', import: 'default', eager: true });

const HEX_COLOR = /#[0-9a-fA-F]{3,8}\b/;
const EXEMPT = /\.test\.tsx?$|\/Locales\/|\/Testing\//;

describe('design tokens', () => {
  it('emits every color token as a theme variable', () => {
    for (const token of tokens.color.tokens) {
      const name = token.name.replace(/^(color|text|border)-/, '');
      expect(generated).toContain(`--color-${name}: ${token.value.dark};`);
    }
  });

  it('defines each theme token exactly once, outside Tokens.json', () => {
    const names = new Set(tokens.color.tokens.map((token) => `--color-${token.name.replace(/^(color|text|border)-/, '')}`));
    for (const token of themes.tokens) expect(names.has(token.variable), token.variable).toBe(false);
  });

  it('puts the default theme on the root and every system theme in its own data-theme block', () => {
    const fallback = themes.themes.find((theme) => theme.id === themes.default);
    expect(fallback).toBeDefined();
    const theme = generated.slice(0, generated.indexOf('\n}\n'));
    for (const token of themes.tokens) expect(theme).toContain(`${token.variable}: ${fallback?.colors[token.key] ?? ''};`);
    for (const entry of themes.themes) {
      const start = generated.indexOf(`[data-theme="${entry.id}"] {`);
      expect(start, entry.id).toBeGreaterThan(-1);
      const block = generated.slice(start, generated.indexOf('\n}', start));
      for (const token of themes.tokens) expect(block).toContain(`${token.variable}: ${entry.colors[token.key] ?? ''};`);
    }
  });

  it('gives every system theme the same 25 hex tokens', () => {
    expect(themes.tokens).toHaveLength(25);
    expect(themes.themes.map((theme) => theme.id)).toEqual(['sea', 'teak', 'dusk', 'indigo']);
    for (const entry of themes.themes) {
      expect(Object.keys(entry.colors).sort()).toEqual(themes.tokens.map((token) => token.key).sort());
      for (const value of Object.values(entry.colors)) expect(value).toMatch(/^#[0-9a-f]{6}$/);
    }
  });

  it('derives the theme-dependent colours from the base tokens, so custom themes follow too', () => {
    expect(generated).toContain('--color-accent: var(--color-aurora-1);');
    expect(generated).toMatch(/--color-on-accent: color-mix\(in srgb, var\(--color-aurora-1\) \d+%, black\);/);
    expect(generated).toMatch(/--color-sel: color-mix\(in srgb, var\(--color-aurora-1\)/);
    for (const tone of ['ok', 'warn', 'bad']) {
      for (const part of ['wash', 'line', 'fg']) expect(generated).toContain(`--color-${tone}-${part}: color-mix(`);
    }
  });

  it('defines the direction A hairlines, radii and control heights', () => {
    expect(generated).toContain('--color-line: #ffffff14;');
    expect(generated).toContain('--color-line-2: #ffffff24;');
    for (const [name, value] of [
      ['sm', '8px'],
      ['md', '12px'],
      ['lg', '16px'],
      ['btn', '10px'],
      ['input', '10px'],
    ]) {
      expect(generated).toContain(`--radius-${name}: ${value};`);
    }
    expect(generated).toContain('--spacing-btn: 40px;');
    expect(generated).toContain('--spacing-input: 44px;');
    expect(generated).toContain('--spacing-menu: 216px;');
  });

  it('keeps every type size at 13px or more', () => {
    const sizes = [...generated.matchAll(/--text-[a-z0-9-]+: (\d+(?:\.\d+)?)px;/g)].map((match) => Number(match[1]));
    expect(sizes.length).toBeGreaterThan(0);
    expect(Math.min(...sizes)).toBeGreaterThanOrEqual(13);
  });

  it('keeps hex colors out of application code', () => {
    const offenders = Object.entries(sources)
      .filter(([path]) => !EXEMPT.test(path))
      .filter(([, source]) => HEX_COLOR.test(source))
      .map(([path]) => path);
    expect(offenders).toEqual([]);
  });
});
