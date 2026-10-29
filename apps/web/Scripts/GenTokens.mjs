import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const source = fileURLToPath(new URL('../Src/Styles/Tokens.json', import.meta.url));
const themeSource = fileURLToPath(new URL('../Src/Styles/Themes.json', import.meta.url));
const target = fileURLToPath(new URL('../Src/Styles/Tokens.css', import.meta.url));
const tokens = JSON.parse(readFileSync(source, 'utf8'));
const themes = JSON.parse(readFileSync(themeSource, 'utf8'));

const HEX = /^#[0-9a-f]{6}$/i;
const DERIVED = /^(color-mix|var)\(/;

const THEME = 'dark';
const COLOR_PREFIX = /^(color|text|border)-/;

const colorValue = (token) => {
  const value = token.value[THEME];
  if (typeof value !== 'string') throw new Error(`token ${token.name} has no ${THEME} value`);
  return value;
};

const assertSpacingGrid = () => {
  for (const token of tokens.spacing.tokens) {
    const step = Number(token.name.replace('space-', ''));
    if (token.value !== `${step * 4}px`) {
      throw new Error(`${token.name} is ${token.value}, which is off the 4px grid Tailwind uses`);
    }
  }
};

const assertThemes = () => {
  const keys = themes.tokens.map((token) => token.key);
  const names = new Set(tokens.color.tokens.map((token) => `--color-${token.name.replace(COLOR_PREFIX, '')}`));
  for (const token of themes.tokens) {
    if (!themes.groups.includes(token.group)) throw new Error(`theme token ${token.key} is in the unknown group ${token.group}`);
    if (names.has(token.variable)) throw new Error(`${token.variable} is defined in both Tokens.json and Themes.json`);
  }
  if (!themes.themes.some((theme) => theme.id === themes.default)) throw new Error(`default theme ${themes.default} is missing`);
  for (const theme of themes.themes) {
    const own = Object.keys(theme.colors);
    if (own.length !== keys.length || !keys.every((key) => own.includes(key))) {
      throw new Error(`theme ${theme.id} must define exactly the ${keys.length} theme tokens`);
    }
    for (const [key, value] of Object.entries(theme.colors)) {
      if (!HEX.test(value)) throw new Error(`theme ${theme.id} token ${key} is ${value}, not #rrggbb`);
    }
  }
};

const themeColors = (theme) => themes.tokens.map((token) => `  ${token.variable}: ${theme.colors[token.key]};`);
const derivedColors = tokens.color.tokens
  .filter((t) => DERIVED.test(colorValue(t)))
  .map((t) => `  --color-${t.name.replace(COLOR_PREFIX, '')}: ${colorValue(t)};`);
const defaultTheme = themes.themes.find((theme) => theme.id === themes.default);
const themeBlocks = themes.themes.map(
  (theme) => `[data-theme="${theme.id}"] {\n${[...themeColors(theme), ...derivedColors].join('\n')}\n}`,
);

const colors = tokens.color.tokens.map((t) => `  --color-${t.name.replace(COLOR_PREFIX, '')}: ${colorValue(t)};`);
const radii = tokens.radius.tokens.map((t) => `  --radius-${t.name.replace('radius-', '')}: ${t.value};`);
const shadows = tokens.shadow.tokens.map((t) => `  --shadow-${t.name.replace('shadow-', '')}: ${t.value};`);
const layoutName = (t) => `--spacing-${t.name.replace('layout-', '')}`;
const layout = [
  ...tokens.layout.tokens.map((t) => `  ${layoutName(t)}: ${t.value};`),
  ...tokens.control.tokens.map((t) => `  --spacing-${t.name.replace('control-', '')}: ${t.value};`),
];
const phoneLayout = tokens.layout.tokens.filter((t) => t.phone !== undefined).map((t) => `  ${layoutName(t)}: ${t.phone};`);
const fonts = Object.entries(tokens.type.families).map(([name, stack]) => `  --font-${name}: ${stack};`);
const styles = tokens.type.groups.flatMap((group) => group.styles);
const text = styles.flatMap((s) => [
  `  --text-${s.name}: ${s.fontSize};`,
  `  --text-${s.name}--line-height: ${s.lineHeight};`,
  `  --text-${s.name}--font-weight: ${s.fontWeight};`,
]);
const arabicText = styles.flatMap((s) => [
  ...(s.arabic.fontSize === s.fontSize ? [] : [`  --text-${s.name}: ${s.arabic.fontSize};`]),
  ...(s.arabic.lineHeight === s.lineHeight ? [] : [`  --text-${s.name}--line-height: ${s.arabic.lineHeight};`]),
]);

const MIN_TEXT = 13;
const MAX_WEIGHT = 600;
const PX = /^(\d+)px$/;

const assertType = () => {
  for (const s of styles) {
    if (!s.arabic || !PX.test(s.arabic.fontSize) || !PX.test(s.arabic.lineHeight)) {
      throw new Error(`type style ${s.name} needs an explicit arabic fontSize and lineHeight in px`);
    }
    for (const size of [s.fontSize, s.arabic.fontSize]) {
      if (Number(PX.exec(size)?.[1]) < MIN_TEXT) throw new Error(`type style ${s.name} is ${size}, under the ${MIN_TEXT}px floor`);
    }
    if (Number(PX.exec(s.arabic.fontSize)?.[1]) < Number(PX.exec(s.fontSize)?.[1])) {
      throw new Error(`type style ${s.name} is smaller in Arabic than in Latin`);
    }
    if (s.fontWeight > MAX_WEIGHT) throw new Error(`type style ${s.name} weighs ${s.fontWeight}; the scale stops at ${MAX_WEIGHT}`);
  }
};

assertSpacingGrid();
assertThemes();
assertType();

const resets = ['color', 'text', 'radius', 'shadow', 'font'].map((ns) => `  --${ns}-*: initial;`);
const body = [...resets, ...colors, ...themeColors(defaultTheme), ...radii, ...shadows, ...layout, ...fonts, ...text].join('\n');
const header = '/* Generated by Scripts/GenTokens.mjs from Tokens.json and Themes.json. Do not edit by hand. */';
const arabic = `:root:lang(ar) {\n${['  --font-sans: var(--font-arabic);', ...arabicText].join('\n')}\n}`;
const phone = `@media (width < ${tokens.layout.phoneBelow}) {\n  :root {\n${phoneLayout.map((line) => `  ${line}`).join('\n')}\n  }\n}`;
const css = `${header}\n@theme static {\n${body}\n}\n${arabic}\n${phone}\n${themeBlocks.join('\n')}\n`;

writeFileSync(target, css);
process.stdout.write(`wrote ${target}\n`);
