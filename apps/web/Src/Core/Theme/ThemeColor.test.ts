import { describe, expect, it } from 'vitest';

import {
  BACKGROUND_TOKENS,
  colorDistance,
  contrastChecks,
  contrastRatio,
  deriveBackgrounds,
  hexToHsl,
  hslToHex,
  onAccent,
  similarStatusPairs,
} from '@/Core/Theme/ThemeColor';
import { DEFAULT_COLORS, SYSTEM_THEMES } from '@/Core/Theme/ThemeTokens';
import { HEX_PATTERN } from '@/Core/Theme/ThemeValidation';

describe('contrast ratio', () => {
  it('matches the WCAG values for known pairs', () => {
    expect(contrastRatio('#000000', '#ffffff')).toBeCloseTo(21, 5);
    expect(contrastRatio('#ffffff', '#ffffff')).toBeCloseTo(1, 5);
    expect(contrastRatio('#767676', '#ffffff')).toBeCloseTo(4.54, 2);
    expect(contrastRatio('#777777', '#ffffff')).toBeCloseTo(4.48, 2);
    expect(contrastRatio('#ff0000', '#000000')).toBeCloseTo(5.25, 2);
  });

  it('does not depend on the order of the two colours', () => {
    expect(contrastRatio('#123456', '#fedcba')).toBe(contrastRatio('#fedcba', '#123456'));
  });
});

describe('contrast checks', () => {
  it('reports the three pairs for every system theme and all of them pass', () => {
    for (const theme of SYSTEM_THEMES) {
      const checks = contrastChecks(theme.Colors);
      expect(checks.map((check) => check.Id)).toEqual(['textOnCards', 'secondaryOnCanvas', 'textOnPrimary']);
      for (const check of checks) expect(check.Ok, `${theme.Id} ${check.Id} ${check.Ratio.toFixed(2)}`).toBe(true);
    }
  });

  it('fails a pair under 4.5 to 1 and passes it at or above', () => {
    const [low] = contrastChecks({ ...DEFAULT_COLORS, t1: '#777777', surface: '#ffffff' });
    expect(low?.Ok).toBe(false);
    const [high] = contrastChecks({ ...DEFAULT_COLORS, t1: '#767676', surface: '#ffffff' });
    expect(high?.Ok).toBe(true);
  });

  it('checks the label on the primary button with the colour derived from a1', () => {
    const [, , primary] = contrastChecks({ ...DEFAULT_COLORS, a1: '#000000' });
    expect(primary?.Ratio).toBeCloseTo(contrastRatio(onAccent('#000000'), '#000000'), 5);
    expect(primary?.Ok).toBe(false);
  });

  it('keeps the primary button label at 4.5 to 1 or better in every system theme', () => {
    for (const theme of SYSTEM_THEMES) {
      const ratio = contrastRatio(onAccent(theme.Colors.a1), theme.Colors.a1);
      expect(ratio, theme.Id).toBeGreaterThanOrEqual(4.5);
    }
  });

  it('derives the label by darkening a1 the same way the CSS does', () => {
    expect(onAccent('#2ec4b6')).toBe('#04100f');
    expect(onAccent('#ffffff')).toBe('#141414');
  });

  it('keeps secondary and tertiary text readable on every surface of every system theme', () => {
    const surfaces = ['canvas', 'rail', 'input', 'inset', 'surface', 'raised', 'top'] as const;
    for (const theme of SYSTEM_THEMES) {
      for (const surface of surfaces) {
        const on = theme.Colors[surface];
        expect(contrastRatio(theme.Colors.t2, on), `${theme.Id} t2 on ${surface}`).toBeGreaterThanOrEqual(7);
        expect(contrastRatio(theme.Colors.t3, on), `${theme.Id} t3 on ${surface}`).toBeGreaterThanOrEqual(4.5);
      }
    }
  });
});

describe('colour conversion', () => {
  it('round-trips through HSL within one step per channel', () => {
    for (const hex of ['#336699', '#f2b66b', '#071318', '#ffffff', '#000000', '#808080']) {
      const [h, s, l] = hexToHsl(hex);
      const back = hslToHex(h, s, l);
      for (const index of [1, 3, 5]) {
        const delta = Math.abs(parseInt(hex.slice(index, index + 2), 16) - parseInt(back.slice(index, index + 2), 16));
        expect(delta).toBeLessThanOrEqual(1);
      }
    }
  });
});

describe('background derivation', () => {
  it('derives all eight backgrounds as valid hex, getting lighter in order', () => {
    const result = deriveBackgrounds('#336699');
    expect(Object.keys(result).sort()).toEqual([...BACKGROUND_TOKENS].sort());
    const order = ['input', 'canvas', 'rail', 'inset', 'surface', 'raised', 'top', 'track'] as const;
    let previous = -1;
    for (const key of order) {
      expect(result[key]).toMatch(HEX_PATTERN);
      const lightness = hexToHsl(result[key])[2];
      expect(lightness).toBeGreaterThan(previous);
      previous = lightness;
    }
  });

  it('keeps the hue and caps the saturation at 45 percent', () => {
    const result = deriveBackgrounds('#0000ff');
    const [hue, saturation] = hexToHsl(result.track);
    expect(hue).toBeCloseTo(240, 0);
    expect(saturation).toBeLessThanOrEqual(46);
  });

  it('turns a grey into greys', () => {
    expect(deriveBackgrounds('#808080')).toMatchObject({ input: '#040404', canvas: '#0c0c0c', track: '#3b3b3b' });
  });
});

describe('status colour similarity', () => {
  it('measures zero for the same colour and about a hundred for black and white', () => {
    expect(colorDistance('#336699', '#336699')).toBe(0);
    expect(colorDistance('#000000', '#ffffff')).toBeCloseTo(100, 0);
  });

  it('does not flag any system theme', () => {
    for (const theme of SYSTEM_THEMES) expect(similarStatusPairs(theme.Colors), theme.Id).toEqual([]);
  });

  it('flags two status colours that are nearly the same', () => {
    const pairs = similarStatusPairs({ ...DEFAULT_COLORS, green: '#6bbf72', blue: '#6cbf74' });
    expect(pairs.map((pair) => [pair.First, pair.Second])).toEqual([['green', 'blue']]);
  });

  it('flags an identical pair once', () => {
    const pairs = similarStatusPairs({ ...DEFAULT_COLORS, coral: '#ed7572', amber: '#ed7572' });
    expect(pairs).toHaveLength(1);
  });
});
