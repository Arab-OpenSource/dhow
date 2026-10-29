import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import { describe, expect, it } from 'vitest';

interface IColorToken {
  name: string;
  value: { dark: string };
}

interface ITypeStyle {
  name: string;
  fontSize: string;
  lineHeight: string;
  fontWeight: number;
  arabic: { fontSize: string; lineHeight: string };
}

interface ITokenFile {
  color: { tokens: IColorToken[] };
  type: { families: Record<string, string>; groups: { styles: ITypeStyle[] }[] };
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
const TEXT_CLASS = /(?<![\w-])text-([\w[\]#.-]+)/g;
const TAILWIND_TEXT = ['start', 'end', 'center', 'justify', 'ellipsis', 'clip', 'wrap', 'nowrap', 'balance', 'pretty', 'inherit', 'current', 'transparent'];

const styles = tokens.type.groups.flatMap((group) => group.styles);
const block = (selector: string): string => {
  const start = generated.indexOf(`${selector} {`);
  return start === -1 ? '' : generated.slice(start, generated.indexOf('\n}', start));
};
const px = (value: string): number => Number(value.replace('px', ''));

const SCALE: [string, string, string, string, string][] = [
  ['title-page', '24px', '34px', '26px', '38px'],
  ['title-section', '18px', '28px', '19px', '30px'],
  ['title-card', '16px', '24px', '17px', '26px'],
  ['body', '15px', '24px', '16px', '28px'],
  ['body-sm', '14px', '22px', '15px', '24px'],
  ['label', '14px', '20px', '15px', '22px'],
  ['label-sm', '13px', '20px', '14px', '20px'],
  ['caption', '13px', '20px', '13px', '20px'],
  ['stat', '30px', '38px', '30px', '38px'],
  ['code', '13px', '22px', '13px', '22px'],
];

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
    for (const tone of ['ok', 'warn', 'bad', 'info', 'queue']) {
      for (const part of ['wash', 'line', 'fg']) expect(generated).toContain(`--color-${tone}-${part}: color-mix(`);
    }
  });

  it('defines the control room hairlines, radii and control heights', () => {
    expect(generated).toContain('--color-line: #ffffff1a;');
    expect(generated).toContain('--color-line-2: #ffffff2e;');
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
    expect(generated).toContain('--spacing-menu: 248px;');
    expect(generated).toContain('--spacing-panel: 560px;');
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

describe('type and spacing scale', () => {
  it.each(SCALE)('sizes %s at %s/%s in Latin and %s/%s in Arabic', (name, size, line, arabicSize, arabicLine) => {
    const style = styles.find((entry) => entry.name === name);
    expect(style).toMatchObject({ fontSize: size, lineHeight: line, arabic: { fontSize: arabicSize, lineHeight: arabicLine } });
    expect(generated).toContain(`--text-${name}: ${size};`);
  });

  it('gives Arabic its own font and sizes on the root, with no class needed', () => {
    const arabic = block(':root:lang(ar)');
    expect(arabic).toContain('--font-sans: var(--font-arabic);');
    for (const style of styles) {
      expect(px(style.arabic.fontSize), style.name).toBeGreaterThanOrEqual(px(style.fontSize));
      if (style.arabic.fontSize !== style.fontSize) expect(arabic).toContain(`--text-${style.name}: ${style.arabic.fontSize};`);
      if (style.arabic.lineHeight !== style.lineHeight) {
        expect(arabic).toContain(`--text-${style.name}--line-height: ${style.arabic.lineHeight};`);
      }
    }
  });

  it('uses IBM Plex for every family and caps the weights at 600', () => {
    expect(tokens.type.families.sans).toMatch(/^"IBM Plex Sans", "IBM Plex Sans Arabic"/);
    expect(tokens.type.families.arabic).toMatch(/^"IBM Plex Sans Arabic", "IBM Plex Sans"/);
    expect(tokens.type.families.mono).toMatch(/^"IBM Plex Mono"/);
    expect(generated).not.toMatch(/poppins/i);
    for (const style of styles) expect(style.fontWeight, style.name).toBeLessThanOrEqual(600);
  });

  it('switches the gutter and card padding to their phone values below md', () => {
    expect(generated).toContain('--spacing-gutter: 32px;');
    expect(generated).toContain('--spacing-card: 20px;');
    expect(generated).toContain('--spacing-block: 16px;');
    expect(generated).toContain('--spacing-topbar: 60px;');
    const phone = generated.slice(generated.indexOf('@media (width < 721px) {'));
    expect(phone).toContain('--spacing-gutter: 16px;');
    expect(phone).toContain('--spacing-card: 14px;');
  });

  it('uses only text classes the tokens define', () => {
    const colors = [
      ...tokens.color.tokens.map((token) => token.name.replace(/^(color|text|border)-/, '')),
      ...themes.tokens.map((token) => token.variable.replace('--color-', '')),
    ];
    const known = new Set([...styles.map((style) => style.name), ...colors, ...TAILWIND_TEXT]);
    const unknown = Object.entries(sources)
      .filter(([path]) => !EXEMPT.test(path))
      .flatMap(([path, source]) => [...source.matchAll(TEXT_CLASS)].map((match) => `${path}: text-${match[1] ?? ''}`))
      .filter((entry) => !known.has(entry.slice(entry.indexOf(': text-') + 7)));
    expect(unknown).toEqual([]);
  });
});
