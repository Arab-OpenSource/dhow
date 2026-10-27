import themes from '@/Styles/Themes.json';

export const THEME_TOKEN_KEYS = [
  'canvas',
  'rail',
  'input',
  'inset',
  'surface',
  'raised',
  'top',
  'track',
  't1',
  't2',
  't3',
  't4',
  'a1',
  'a2',
  'a3',
  'a4',
  'a5',
  'a6',
  'edge-warm',
  'edge-cool',
  'green',
  'blue',
  'amber',
  'coral',
  'violet-soft',
] as const;

export type TThemeToken = (typeof THEME_TOKEN_KEYS)[number];
export type TThemeColors = Record<TThemeToken, string>;

export const THEME_GROUPS = ['bg', 'text', 'aurora', 'edge', 'status'] as const;
export type TThemeGroup = (typeof THEME_GROUPS)[number];

export const SYSTEM_THEME_IDS = ['sea', 'teak', 'dusk', 'indigo'] as const;
export type TSystemThemeId = (typeof SYSTEM_THEME_IDS)[number];

export const DEFAULT_THEME_ID: TSystemThemeId = 'sea';

export const STATUS_TOKENS = ['green', 'blue', 'amber', 'coral', 'violet-soft'] as const satisfies readonly TThemeToken[];

export interface ISystemTheme {
  Id: TSystemThemeId;
  Colors: TThemeColors;
}

export interface ICustomTheme {
  Id: string;
  Name: string;
  Colors: TThemeColors;
}

export interface IThemeTokenGroup {
  Group: TThemeGroup;
  Tokens: TThemeToken[];
}

export const isSystemThemeId = (value: unknown): value is TSystemThemeId =>
  SYSTEM_THEME_IDS.some((id) => id === value);

export const THEME_VARIABLES = Object.fromEntries(themes.tokens.map((token) => [token.key, token.variable])) as Record<
  TThemeToken,
  string
>;

const GROUP_OF = Object.fromEntries(themes.tokens.map((token) => [token.key, token.group])) as Record<
  TThemeToken,
  TThemeGroup
>;

export const THEME_TOKEN_GROUPS: IThemeTokenGroup[] = THEME_GROUPS.map((Group) => ({
  Group,
  Tokens: THEME_TOKEN_KEYS.filter((key) => GROUP_OF[key] === Group),
}));

export const SYSTEM_THEMES: ISystemTheme[] = themes.themes.flatMap((theme) =>
  isSystemThemeId(theme.id) ? [{ Id: theme.id, Colors: theme.colors }] : [],
);

const defaultTheme = SYSTEM_THEMES.find((theme) => theme.Id === DEFAULT_THEME_ID);
if (!defaultTheme) throw new Error(`Themes.json does not define the default theme ${DEFAULT_THEME_ID}`);

export const DEFAULT_COLORS: TThemeColors = defaultTheme.Colors;
