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

const read = (name: string): string => readFileSync(fileURLToPath(new URL(name, import.meta.url)), 'utf8');

const tokens = JSON.parse(read('./Tokens.json')) as ITokenFile;
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

  it('keeps hex colors out of application code', () => {
    const offenders = Object.entries(sources)
      .filter(([path]) => !EXEMPT.test(path))
      .filter(([, source]) => HEX_COLOR.test(source))
      .map(([path]) => path);
    expect(offenders).toEqual([]);
  });
});
