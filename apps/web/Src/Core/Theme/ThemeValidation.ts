import { isCustomThemeId } from '@/Core/Theme/ThemeIds';
import { DEFAULT_COLORS, THEME_TOKEN_KEYS, type ICustomTheme, type TThemeColors } from '@/Core/Theme/ThemeTokens';

export { isCustomThemeId, THEME_ID_PATTERN } from '@/Core/Theme/ThemeIds';

export const HEX_PATTERN = /^#[0-9a-f]{6}$/i;
export const THEME_NAME_MAX = 40;
export const MAX_CUSTOM_THEMES = 50;

const CONTROL_CHARACTERS = /\p{Cc}/gu;

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

export const isHex = (value: unknown): value is string => typeof value === 'string' && HEX_PATTERN.test(value);

export const cleanThemeName = (value: unknown): string | null => {
  if (typeof value !== 'string') return null;
  const cleaned = value.replace(CONTROL_CHARACTERS, '').trim().slice(0, THEME_NAME_MAX).trim();
  return cleaned === '' ? null : cleaned;
};

export const sanitizeColors = (input: unknown, fallback: TThemeColors = DEFAULT_COLORS): TThemeColors => {
  const source = isRecord(input) ? input : {};
  return Object.fromEntries(
    THEME_TOKEN_KEYS.map((key) => {
      const value = Object.hasOwn(source, key) ? source[key] : undefined;
      return [key, isHex(value) ? value.toLowerCase() : fallback[key]];
    }),
  ) as TThemeColors;
};

export const sanitizeCustomTheme = (input: unknown): ICustomTheme | null => {
  if (!isRecord(input)) return null;
  const { Id, Name, Colors } = input;
  const name = cleanThemeName(Name);
  if (!isCustomThemeId(Id) || name === null || !isRecord(Colors)) return null;
  return { Id, Name: name, Colors: sanitizeColors(Colors) };
};

export const sanitizeCustomThemes = (input: unknown): ICustomTheme[] => {
  if (!Array.isArray(input)) return [];
  const seen = new Set<string>();
  const themes: ICustomTheme[] = [];
  for (const entry of input as unknown[]) {
    const theme = sanitizeCustomTheme(entry);
    if (theme && !seen.has(theme.Id) && themes.length < MAX_CUSTOM_THEMES) {
      seen.add(theme.Id);
      themes.push(theme);
    }
  }
  return themes;
};
