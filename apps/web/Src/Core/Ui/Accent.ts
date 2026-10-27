import type { TIconName } from '@/Core/Ui/Icons';

export type TAccent = 'live' | 'building' | 'awaiting' | 'failed' | 'queued';

export const ACCENT_TEXT: Record<TAccent, string> = {
  live: 'text-accent-green',
  building: 'text-accent-blue',
  awaiting: 'text-accent-amber',
  failed: 'text-accent-coral',
  queued: 'text-accent-violet',
};

export const ACCENT_DOT: Record<TAccent, string> = {
  live: 'bg-accent-green',
  building: 'bg-accent-blue',
  awaiting: 'bg-accent-amber',
  failed: 'bg-accent-coral',
  queued: 'bg-accent-violet',
};

export const ACCENT_SURFACE: Record<TAccent, string> = {
  live: 'border-ok-line bg-ok-wash',
  building: 'border-line bg-surface-inset',
  awaiting: 'border-warn-line bg-warn-wash',
  failed: 'border-bad-line bg-bad-wash',
  queued: 'border-line bg-surface-inset',
};

export const ACCENT_ICON: Record<TAccent, TIconName> = {
  live: 'check',
  building: 'info',
  awaiting: 'alert',
  failed: 'alert',
  queued: 'queue',
};
