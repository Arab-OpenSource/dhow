import type { ReactNode } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import type { TIconName } from '@/Core/Ui/Icons';
import type { TTone } from '@/Core/Ui/Tone';

export type TBadgeTone = TTone | 'mute';

export interface IBadgeProps {
  tone?: TBadgeTone;
  icon?: TIconName;
  children: ReactNode;
  className?: string;
}

const TONES: Record<TBadgeTone, string> = {
  ok: 'bg-ok-wash text-ok-fg',
  warn: 'bg-warn-wash text-warn-fg',
  bad: 'bg-bad-wash text-bad-fg',
  mute: 'bg-wash text-secondary',
};

export const Badge = ({ tone = 'ok', icon, children, className }: IBadgeProps) => (
  <span
    data-tone={tone}
    className={cx('inline-flex items-center gap-1.5 rounded-pill px-2.5 py-0.5 text-caption whitespace-nowrap', TONES[tone], className)}
  >
    {icon ? <Icon name={icon} size={13} weight={2.4} /> : null}
    {children}
  </span>
);
