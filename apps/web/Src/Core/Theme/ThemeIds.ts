export const SYSTEM_THEME_IDS = ['sea', 'teak', 'dusk', 'indigo'] as const;
export type TSystemThemeId = (typeof SYSTEM_THEME_IDS)[number];

export const DEFAULT_THEME_ID: TSystemThemeId = 'sea';

export const THEME_ID_PATTERN = /^[a-z0-9-]{1,40}$/;

export const isSystemThemeId = (value: unknown): value is TSystemThemeId => SYSTEM_THEME_IDS.some((id) => id === value);

export const isCustomThemeId = (value: unknown): value is string =>
  typeof value === 'string' && THEME_ID_PATTERN.test(value) && !isSystemThemeId(value);
