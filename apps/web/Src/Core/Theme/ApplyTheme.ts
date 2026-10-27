import {
  DEFAULT_THEME_ID,
  SYSTEM_THEMES,
  THEME_TOKEN_KEYS,
  THEME_VARIABLES,
  type ICustomTheme,
  type TThemeColors,
} from '@/Core/Theme/ThemeTokens';
import { isHex } from '@/Core/Theme/ThemeValidation';

const syncBrowserColor = (canvas: string): void => {
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', canvas);
};

export const clearInlineColors = (root: HTMLElement = document.documentElement): void => {
  for (const key of THEME_TOKEN_KEYS) root.style.removeProperty(THEME_VARIABLES[key]);
};

export const applyColors = (colors: TThemeColors, root: HTMLElement = document.documentElement): void => {
  for (const key of THEME_TOKEN_KEYS) {
    const value = colors[key];
    if (isHex(value)) root.style.setProperty(THEME_VARIABLES[key], value);
  }
  if (isHex(colors.canvas)) syncBrowserColor(colors.canvas);
};

export const applyTheme = (
  id: string,
  custom: readonly ICustomTheme[],
  root: HTMLElement = document.documentElement,
): string => {
  const system = SYSTEM_THEMES.find((theme) => theme.Id === id);
  if (system) {
    clearInlineColors(root);
    root.setAttribute('data-theme', system.Id);
    syncBrowserColor(system.Colors.canvas);
    return system.Id;
  }
  const mine = custom.find((theme) => theme.Id === id);
  if (mine) {
    root.removeAttribute('data-theme');
    applyColors(mine.Colors, root);
    return mine.Id;
  }
  return applyTheme(DEFAULT_THEME_ID, custom, root);
};

export const findThemeColors = (id: string, custom: readonly ICustomTheme[]): TThemeColors | undefined =>
  SYSTEM_THEMES.find((theme) => theme.Id === id)?.Colors ?? custom.find((theme) => theme.Id === id)?.Colors;
