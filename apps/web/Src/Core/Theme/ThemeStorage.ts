import { DEFAULT_THEME_ID, isSystemThemeId, type ICustomTheme } from '@/Core/Theme/ThemeTokens';
import { isCustomThemeId, sanitizeCustomThemes } from '@/Core/Theme/ThemeValidation';

export const ACTIVE_THEME_KEY = 'dhow.theme';
export const CUSTOM_THEMES_KEY = 'dhow.themes';

export interface IStoredThemes {
  ActiveId: string;
  CustomThemes: ICustomTheme[];
}

const readItem = (key: string): string | null => {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
};

const writeItem = (key: string, value: string): void => {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    return;
  }
};

const parseJson = (raw: string | null): unknown => {
  if (raw === null) return [];
  try {
    return JSON.parse(raw) as unknown;
  } catch {
    return [];
  }
};

export const readStoredThemes = (): IStoredThemes => {
  const CustomThemes = sanitizeCustomThemes(parseJson(readItem(CUSTOM_THEMES_KEY)));
  const stored = readItem(ACTIVE_THEME_KEY);
  const known =
    stored !== null &&
    (isSystemThemeId(stored) || (isCustomThemeId(stored) && CustomThemes.some((theme) => theme.Id === stored)));
  return { ActiveId: known ? stored : DEFAULT_THEME_ID, CustomThemes };
};

export const storeActiveTheme = (id: string): void => {
  writeItem(ACTIVE_THEME_KEY, id);
};

export const storeCustomThemes = (themes: readonly ICustomTheme[]): void => {
  writeItem(CUSTOM_THEMES_KEY, JSON.stringify(themes));
};
