import type { ReactNode } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import type { TIconName } from '@/Core/Ui/Icons';
import type { TStatusTone } from '@/Core/Ui/Tone';

export type TBadgeTone = TStatusTone | 'mute';

export interface IBadgeProps {
  tone?: TBadgeTone;
  icon?: TIconName;
  children: ReactNode;
  className?: string;
}

const TONES: Record<TBadgeTone, string> = {
  ok: 'border-ok-line bg-ok-wash text-ok-fg',
  info: 'border-info-line bg-info-wash text-info-fg',
  queue: 'border-queue-line bg-queue-wash text-queue-fg',
  warn: 'border-warn-line bg-warn-wash text-warn-fg',
  bad: 'border-bad-line bg-bad-wash text-bad-fg',
  mute: 'border-line bg-wash text-secondary',
};

export const Badge = ({ tone = 'ok', icon, children, className }: IBadgeProps) => (
  <span
    data-tone={tone}
    className={cx('inline-flex h-6.5 items-center gap-1.5 rounded-pill border px-2.5 text-caption font-medium whitespace-nowrap', TONES[tone], className)}
  >
    {icon ? <Icon name={icon} size={13} weight={2.4} /> : null}
    {children}
  </span>
);
