import type { TBadgeTone } from '@/Core/Ui/Badge';
import type { TStatus } from '@/Core/Ui/StatusDot';
import type { TMessageKey } from '@/I18n/Messages';
import type { TRegistrationTokenStatus, TRunner } from '@/Modules/Runners/Api/RunnersApi';

export type TRunnerView = 'online' | 'offline' | 'paused' | 'cloneSuspected' | 'revoked';

export const RUNNER_NAME_MAX = 100;
export const TOKEN_DESCRIPTION_MAX = 200;
export const LABELS_MAX = 32;
export const MAX_USES_LIMIT = 50;
export const EXPIRY_HOURS = ['1', '6', '24'] as const;
export type TExpiryHours = (typeof EXPIRY_HOURS)[number];
export const DEFAULT_EXPIRY: TExpiryHours = '1';

const SECONDS_PER_HOUR = 3600;
const LABEL_PATTERN = /^[a-z0-9]([a-z0-9._-]{0,62}[a-z0-9])?(=[A-Za-z0-9]([A-Za-z0-9._-]{0,62}[A-Za-z0-9])?)?$/;
export const RESERVED_PREFIX = 'dhow.';
export const LABEL_EXAMPLE = 'linux-builders region=eu';
const LABEL_SEPARATOR = /[\s,]+/;

export const viewOf = (runner: TRunner): TRunnerView => {
  if (runner.State === 'revoked') return 'revoked';
  if (runner.State === 'disabled') return runner.DisabledReason === 'clone_suspected' ? 'cloneSuspected' : 'paused';
  return runner.Status === 'online' ? 'online' : 'offline';
};

export const VIEW_DOT: Record<TRunnerView, TStatus> = {
  online: 'live',
  offline: 'rolledback',
  paused: 'awaiting',
  cloneSuspected: 'failed',
  revoked: 'rolledback',
};

export const VIEW_TONE: Record<TRunnerView, TBadgeTone> = {
  online: 'ok',
  offline: 'mute',
  paused: 'warn',
  cloneSuspected: 'bad',
  revoked: 'mute',
};

export const VIEW_LABEL: Record<TRunnerView, TMessageKey> = {
  online: 'runners.view.online',
  offline: 'runners.view.offline',
  paused: 'runners.view.paused',
  cloneSuspected: 'runners.view.cloneSuspected',
  revoked: 'runners.view.revoked',
};

export const TOKEN_TONE: Record<TRegistrationTokenStatus, TBadgeTone> = {
  active: 'ok',
  expired: 'mute',
  used_up: 'mute',
  revoked: 'mute',
};

export const TOKEN_LABEL: Record<TRegistrationTokenStatus, TMessageKey> = {
  active: 'runners.token.status.active',
  expired: 'runners.token.status.expired',
  used_up: 'runners.token.status.usedUp',
  revoked: 'runners.token.status.revoked',
};

export const splitLabels = (raw: string): string[] => [
  ...new Set(
    raw
      .split(LABEL_SEPARATOR)
      .map((label) => label.trim())
      .filter((label) => label !== ''),
  ),
];

export const labelsValid = (labels: readonly string[]): boolean =>
  labels.length <= LABELS_MAX && labels.every((label) => LABEL_PATTERN.test(label) && !label.startsWith(RESERVED_PREFIX));

export const expirySeconds = (hours: TExpiryHours): number => Number(hours) * SECONDS_PER_HOUR;

export const sameLabels = (a: readonly string[], b: readonly string[]): boolean =>
  a.length === b.length && [...a].sort().every((label, index) => label === [...b].sort()[index]);

export const registerCommandFor = (origin: string): string => `sudo dhow-runner register --url ${origin} --token-file -`;
