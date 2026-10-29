import { STATUS_TOKENS, type TThemeColors, type TThemeToken } from '@/Core/Theme/ThemeTokens';
import { color } from '@/Styles/Tokens.json';

export const MIN_CONTRAST = 4.5;
export const MIN_STATUS_DISTANCE = 15;
export const BACKGROUND_TOKENS = ['input', 'canvas', 'rail', 'inset', 'surface', 'raised', 'top', 'track'] as const;
export type TBackgroundToken = (typeof BACKGROUND_TOKENS)[number];

const BACKGROUND_LIGHTNESS: Record<TBackgroundToken, number> = {
  input: 1.7,
  canvas: 4.7,
  rail: 6.7,
  inset: 7.8,
  surface: 10.1,
  raised: 14.8,
  top: 17,
  track: 23,
};
const MAX_SATURATION = 45;

const onAccentToken = color.tokens.find((token) => token.name === 'color-on-accent');
const onAccentShare = /color-mix\(in srgb, var\(--color-aurora-1\) (\d+)%, black\)/.exec(onAccentToken?.value.dark ?? '')?.[1];
if (onAccentShare === undefined) throw new Error('Tokens.json does not define color-on-accent as a mix of a1 with black');
const ON_ACCENT_SHARE = Number(onAccentShare) / 100;

type TRgb = [number, number, number];
type TLab = [number, number, number];

const channels = (hex: string): TRgb => [
  parseInt(hex.slice(1, 3), 16),
  parseInt(hex.slice(3, 5), 16),
  parseInt(hex.slice(5, 7), 16),
];

const linear = (channel: number): number => {
  const value = channel / 255;
  return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
};

export const relativeLuminance = (hex: string): number => {
  const [r, g, b] = channels(hex);
  return 0.2126 * linear(r) + 0.7152 * linear(g) + 0.0722 * linear(b);
};

export const contrastRatio = (first: string, second: string): number => {
  const [lighter, darker] = [relativeLuminance(first), relativeLuminance(second)].sort((a, b) => b - a);
  return ((lighter ?? 0) + 0.05) / ((darker ?? 0) + 0.05);
};

export const onAccent = (accent: string): string =>
  `#${channels(accent)
    .map((value) =>
      Math.round(value * ON_ACCENT_SHARE)
        .toString(16)
        .padStart(2, '0'),
    )
    .join('')}`;

export const hexToHsl = (hex: string): [number, number, number] => {
  const [r, g, b] = channels(hex).map((value) => value / 255) as TRgb;
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const lightness = (max + min) / 2;
  if (max === min) return [0, 0, lightness * 100];
  const delta = max - min;
  const saturation = lightness > 0.5 ? delta / (2 - max - min) : delta / (max + min);
  let hue: number;
  if (max === r) hue = (g - b) / delta + (g < b ? 6 : 0);
  else if (max === g) hue = (b - r) / delta + 2;
  else hue = (r - g) / delta + 4;
  return [(hue / 6) * 360, saturation * 100, lightness * 100];
};

export const hslToHex = (hue: number, saturation: number, lightness: number): string => {
  const s = saturation / 100;
  const l = lightness / 100;
  const a = s * Math.min(l, 1 - l);
  const channel = (n: number): string => {
    const k = (n + hue / 30) % 12;
    const value = l - a * Math.max(-1, Math.min(k - 3, Math.min(9 - k, 1)));
    return Math.round(255 * value)
      .toString(16)
      .padStart(2, '0');
  };
  return `#${channel(0)}${channel(8)}${channel(4)}`;
};

export const deriveBackgrounds = (hex: string): Record<TBackgroundToken, string> => {
  const [hue, saturation] = hexToHsl(hex);
  const sat = Math.min(saturation, MAX_SATURATION);
  return Object.fromEntries(
    BACKGROUND_TOKENS.map((key) => [key, hslToHex(hue, sat, BACKGROUND_LIGHTNESS[key])]),
  ) as Record<TBackgroundToken, string>;
};

const toLab = (hex: string): TLab => {
  const [r, g, b] = channels(hex).map(linear) as TRgb;
  const x = (0.4124 * r + 0.3576 * g + 0.1805 * b) / 0.95047;
  const y = 0.2126 * r + 0.7152 * g + 0.0722 * b;
  const z = (0.0193 * r + 0.1192 * g + 0.9505 * b) / 1.08883;
  const f = (value: number): number => (value > 0.008856 ? Math.cbrt(value) : 7.787 * value + 16 / 116);
  return [116 * f(y) - 16, 500 * (f(x) - f(y)), 200 * (f(y) - f(z))];
};

export const colorDistance = (first: string, second: string): number => {
  const [l1, a1, b1] = toLab(first);
  const [l2, a2, b2] = toLab(second);
  return Math.hypot(l1 - l2, a1 - a2, b1 - b2);
};

export type TContrastCheckId = 'textOnCards' | 'secondaryOnCanvas' | 'textOnPrimary';

export interface IContrastCheck {
  Id: TContrastCheckId;
  Ratio: number;
  Ok: boolean;
}

export const contrastChecks = (colors: TThemeColors): IContrastCheck[] =>
  (
    [
      ['textOnCards', colors.t1, colors.surface],
      ['secondaryOnCanvas', colors.t2, colors.canvas],
      ['textOnPrimary', onAccent(colors.a1), colors.a1],
    ] as const
  ).map(([Id, foreground, background]) => {
    const Ratio = contrastRatio(foreground, background);
    return { Id, Ratio, Ok: Ratio >= MIN_CONTRAST };
  });

export interface IStatusPair {
  First: TThemeToken;
  Second: TThemeToken;
  Distance: number;
}

export const similarStatusPairs = (colors: TThemeColors): IStatusPair[] => {
  const pairs: IStatusPair[] = [];
  STATUS_TOKENS.forEach((First, index) => {
    for (const Second of STATUS_TOKENS.slice(index + 1)) {
      const Distance = colorDistance(colors[First], colors[Second]);
      if (Distance < MIN_STATUS_DISTANCE) pairs.push({ First, Second, Distance });
    }
  });
  return pairs;
};
