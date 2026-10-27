import { describe, expect, it } from 'vitest';

const sources = import.meta.glob<string>(
  ['../../App/**/*.{ts,tsx}', '../../Core/Ui/**/*.{ts,tsx}', '../../Modules/**/*.{ts,tsx}', '../../Styles/Globals.css'],
  { query: '?raw', import: 'default', eager: true },
);

const EXEMPT = /\.test\.tsx?$|(^|\/)Mark\.tsx$/;

const FORBIDDEN: [string, RegExp][] = [
  ['a gradient', /gradient/i],
  ['the edge-light utility', /(?<![\w.-])edge(?![\w-])/],
  ['an aurora, orb or row fill utility', /(?<![\w-])bg-(aurora|orb|row|card-gradient)(?![\w-])/],
  ['an aurora token used directly', /(?<![\w-])(bg|text|border|from|via|to)-aurora-\d/],
  ['the on-light token', /(?<![\w-])(text|bg)-on-light(?![\w-])/],
];

const files = Object.entries(sources).filter(([path]) => !EXEMPT.test(path));

describe('direction A: no gradients outside the logo', () => {
  it('scans the shell, Core/Ui, the modules and the global stylesheet', () => {
    const paths = files.map(([path]) => path);
    expect(paths.some((path) => path.endsWith('/Button.tsx'))).toBe(true);
    expect(paths.some((path) => path.includes('/Modules/Appearance/'))).toBe(true);
    expect(paths.some((path) => path.endsWith('/Styles/Globals.css'))).toBe(true);
    expect(paths.some((path) => path.endsWith('/Mark.tsx'))).toBe(false);
  });

  it.each(FORBIDDEN)('finds no use of %s', (_name, pattern) => {
    const offenders = files.filter(([, source]) => pattern.test(source)).map(([path]) => path);
    expect(offenders).toEqual([]);
  });

  it('keeps the gradient in the logo mark only', async () => {
    const mark = await import('@/Core/Ui/Mark?raw');
    expect(mark.default).toContain('linearGradient');
  });
});
