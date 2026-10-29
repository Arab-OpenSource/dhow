import { isCustomThemeId } from '@/Core/Theme/ThemeIds';
import { isCheckName } from '@/Modules/System/Types/SystemTypes';

const STACK_SEPARATOR = ',';
const MAX_STACK_DEPTH = 6;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export const LEGACY_HOME_HREF = '/?slide=unknown';

const STATIC_FRAMES: Record<string, string> = {
  'system-status': '/system',
  notifications: '/notifications',
  appearance: '/account/preferences',
  'theme-edit': '/account/preferences?slide=theme-edit',
  account: '/account/profile',
  'account-password': '/account/security?slide=account-password',
  'account-totp': '/account/security?slide=account-totp',
  'account-token-new': '/account/profile?slide=account-token-new',
  audit: '/audit',
  jobs: '/jobs',
  organization: '/org/members',
  'organization-tokens': '/org/api-keys',
  'organization-token-new': '/org/api-keys?slide=organization-token-new',
  organizations: '/organizations',
  'organization-new': '/organizations?slide=organization-new',
  'organization-invite': '/org/members?slide=organization-invite',
  'organization-transfer': '/org/settings?slide=organization-transfer',
  'team-new': '/org/teams?slide=team-new',
  runners: '/runners',
  'runner-new': '/runners?slide=runner-new',
};

interface IPrefixFrame {
  prefix: string;
  valid: (rest: string) => boolean;
  href: (rest: string) => string;
}

const PREFIX_FRAMES: readonly IPrefixFrame[] = [
  { prefix: 'check-', valid: isCheckName, href: (name) => `/system/checks/${name}` },
  { prefix: 'theme-edit-', valid: isCustomThemeId, href: (id) => `/account/preferences?slide=theme-edit-${id}` },
  { prefix: 'member-', valid: (id) => UUID.test(id), href: (id) => `/org/members/${id}` },
  { prefix: 'team-', valid: (id) => UUID.test(id), href: (id) => `/org/teams/${id}` },
  { prefix: 'audit-event-', valid: (id) => UUID.test(id), href: (id) => `/audit/${id}` },
  { prefix: 'job-', valid: (id) => UUID.test(id), href: (id) => `/jobs/${id}` },
  { prefix: 'runner-', valid: (id) => UUID.test(id), href: (id) => `/runners/${id}` },
];

export const legacyFrameHref = (frame: string): string | null => {
  if (Object.hasOwn(STATIC_FRAMES, frame)) return STATIC_FRAMES[frame] ?? null;
  for (const { prefix, valid, href } of PREFIX_FRAMES) {
    if (frame.startsWith(prefix)) {
      const rest = frame.slice(prefix.length);
      return valid(rest) ? href(rest) : null;
    }
  }
  return null;
};

export const legacyStackHref = (stack: string): string => {
  const frames = stack.split(STACK_SEPARATOR);
  if (frames.length > MAX_STACK_DEPTH || new Set(frames).size !== frames.length) return LEGACY_HOME_HREF;
  const targets = frames.map(legacyFrameHref);
  const last = targets.at(-1);
  if (last === undefined || last === null || targets.some((target) => target === null)) return LEGACY_HOME_HREF;
  return last;
};
