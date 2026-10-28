import { defaultStringifySearch } from '@tanstack/react-router';

import { isCustomThemeId } from '@/Core/Theme/ThemeIds';
import { isCheckName, type TCheckName } from '@/Modules/System/Types/SystemTypes';

export const STACK_SEPARATOR = ',';
export const MAX_STACK_DEPTH = 6;
export const UNKNOWN_STACK = 'unknown';

const STATIC_FRAMES = [
  'system-status',
  'notifications',
  'appearance',
  'theme-edit',
  'account',
  'account-password',
  'account-totp',
  'account-token-new',
  'audit',
  'organization',
  'organizations',
  'organization-new',
  'organization-invite',
  'organization-transfer',
  'team-new',
] as const;
const CHECK_PREFIX = 'check-';
const THEME_EDIT_PREFIX = 'theme-edit-';
const MEMBER_PREFIX = 'member-';
const TEAM_PREFIX = 'team-';
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export type TStaticFrameId = (typeof STATIC_FRAMES)[number];
export type TCheckFrameId = `check-${TCheckName}`;
export type TThemeEditFrameId = `theme-edit-${string}`;
export type TMemberFrameId = `member-${string}`;
export type TTeamFrameId = `team-${string}`;
export type TFrameId = TStaticFrameId | TCheckFrameId | TThemeEditFrameId | TMemberFrameId | TTeamFrameId;

export interface IShellSearch {
  stack?: TFrameId[] | typeof UNKNOWN_STACK;
}

export const checkFrame = (name: TCheckName): TCheckFrameId => `${CHECK_PREFIX}${name}`;
export const themeEditFrame = (id: string): TThemeEditFrameId => `${THEME_EDIT_PREFIX}${id}`;
export const memberFrame = (id: string): TMemberFrameId => `${MEMBER_PREFIX}${id}`;
export const teamFrame = (id: string): TTeamFrameId => `${TEAM_PREFIX}${id}`;

export const isFrameId = (value: unknown): value is TFrameId => {
  if (typeof value !== 'string') return false;
  if (STATIC_FRAMES.some((id) => id === value)) return true;
  if (value.startsWith(CHECK_PREFIX)) return isCheckName(value.slice(CHECK_PREFIX.length));
  if (value.startsWith(THEME_EDIT_PREFIX)) return isCustomThemeId(value.slice(THEME_EDIT_PREFIX.length));
  if (value.startsWith(MEMBER_PREFIX)) return UUID.test(value.slice(MEMBER_PREFIX.length));
  if (value.startsWith(TEAM_PREFIX)) return UUID.test(value.slice(TEAM_PREFIX.length));
  return false;
};

export const memberOf = (id: TFrameId): string | null => {
  const rest = id.startsWith(MEMBER_PREFIX) ? id.slice(MEMBER_PREFIX.length) : '';
  return UUID.test(rest) ? rest : null;
};

export const teamOf = (id: TFrameId): string | null => {
  const rest = id.startsWith(TEAM_PREFIX) ? id.slice(TEAM_PREFIX.length) : '';
  return UUID.test(rest) ? rest : null;
};

export const checkOf = (id: TFrameId): TCheckName | null => {
  const name = id.startsWith(CHECK_PREFIX) ? id.slice(CHECK_PREFIX.length) : '';
  return isCheckName(name) ? name : null;
};

export const themeOf = (id: TFrameId): string | null =>
  id.startsWith(THEME_EDIT_PREFIX) ? id.slice(THEME_EDIT_PREFIX.length) : null;

export const isThemeEditFrame = (id: TFrameId): boolean => id === 'theme-edit' || id.startsWith(THEME_EDIT_PREFIX);

export const parseStack = (raw: unknown): TFrameId[] | null => {
  const parts: unknown[] | null =
    typeof raw === 'string' ? raw.split(STACK_SEPARATOR) : Array.isArray(raw) ? raw : null;
  if (parts === null || parts.length === 0 || parts.length > MAX_STACK_DEPTH || new Set(parts).size !== parts.length) {
    return null;
  }
  const frames = parts.filter(isFrameId);
  if (frames.length !== parts.length) return null;
  return frames.filter(isThemeEditFrame).length > 1 ? null : frames;
};

export const parseShellSearch = (search: Record<string, unknown>): IShellSearch => {
  const raw = search.stack;
  if (raw === undefined || raw === '') return {};
  return { stack: parseStack(raw) ?? UNKNOWN_STACK };
};

const framesOf = (stack: unknown): TFrameId[] => parseStack(stack) ?? [];

export const stringifyShellSearch = (search: Record<string, unknown>): string => {
  const { stack, ...rest } = search;
  const base = defaultStringifySearch(rest);
  const frames = framesOf(stack);
  if (frames.length === 0) return base;
  const part = `stack=${frames.join(STACK_SEPARATOR)}`;
  return base === '' ? `?${part}` : `${base}&${part}`;
};
