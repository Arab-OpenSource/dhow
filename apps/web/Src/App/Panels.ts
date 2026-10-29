import { canManageMembers, canManageRunners, canManageTokens, type TRole } from '@/Core/Auth/AuthTypes';
import { isCustomThemeId } from '@/Core/Theme/ThemeIds';

export const UNKNOWN_SLIDE = 'unknown';

const STATIC_SLIDES = [
  'runner-new',
  'organization-invite',
  'organization-transfer',
  'organization-token-new',
  'organization-new',
  'team-new',
  'account-password',
  'account-totp',
  'account-token-new',
  'theme-edit',
] as const;
const THEME_EDIT_PREFIX = 'theme-edit-';

export type TStaticSlideId = (typeof STATIC_SLIDES)[number];
export type TThemeEditSlideId = `theme-edit-${string}`;
export type TSlideId = TStaticSlideId | TThemeEditSlideId;

export interface IShellSearch {
  slide?: TSlideId | typeof UNKNOWN_SLIDE;
  stack?: string;
}

export const themeEditSlide = (id: string): TThemeEditSlideId => `${THEME_EDIT_PREFIX}${id}`;

export const isSlideId = (value: unknown): value is TSlideId => {
  if (typeof value !== 'string') return false;
  if (STATIC_SLIDES.some((id) => id === value)) return true;
  return value.startsWith(THEME_EDIT_PREFIX) && isCustomThemeId(value.slice(THEME_EDIT_PREFIX.length));
};

export const themeOf = (id: TSlideId): string | null =>
  id.startsWith(THEME_EDIT_PREFIX) ? id.slice(THEME_EDIT_PREFIX.length) : null;

export const isThemeEditSlide = (id: TSlideId): boolean => id === 'theme-edit' || id.startsWith(THEME_EDIT_PREFIX);

const SLIDE_GATES: Partial<Record<TStaticSlideId, (role: TRole | undefined) => boolean>> = {
  'runner-new': canManageRunners,
  'organization-invite': canManageMembers,
  'team-new': canManageMembers,
  'organization-token-new': canManageTokens,
  'organization-transfer': (role) => role === 'owner',
};

const isStaticSlide = (id: TSlideId): id is TStaticSlideId => STATIC_SLIDES.some((slide) => slide === id);

export const slideAllowed = (id: TSlideId, role: TRole | undefined): boolean => {
  const gate = isStaticSlide(id) ? SLIDE_GATES[id] : undefined;
  return gate === undefined || gate(role);
};

const legacyStack = (raw: unknown): string | undefined => {
  if (raw === undefined || raw === '') return undefined;
  if (Array.isArray(raw)) return raw.map(String).join(',');
  return typeof raw === 'string' ? raw : JSON.stringify(raw);
};

export const parseShellSearch = (search: Record<string, unknown>): IShellSearch => {
  const result: IShellSearch = {};
  const stack = legacyStack(search.stack);
  if (stack !== undefined) result.stack = stack;
  const raw = search.slide;
  if (raw !== undefined && raw !== '') result.slide = isSlideId(raw) ? raw : UNKNOWN_SLIDE;
  return result;
};
