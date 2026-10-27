import type { ReactNode } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import type { TIconName } from '@/Core/Ui/Icons';
import { TONE_SURFACE, TONE_TEXT, type TTone } from '@/Core/Ui/Tone';

export interface ISummaryProps {
  tone?: TTone;
  icon: TIconName;
  title: string;
  sub?: ReactNode;
  className?: string;
}

export const Summary = ({ tone = 'ok', icon, title, sub, className }: ISummaryProps) => (
  <div data-tone={tone} className={cx('flex items-center gap-3 rounded-md border px-4 py-3.5', TONE_SURFACE[tone], className)}>
    <Icon name={icon} size={22} className={cx('flex-none', TONE_TEXT[tone])} />
    <div className="grid min-w-0 gap-px">
      <b className="text-title-sm">{title}</b>
      {sub ? <small className="text-caption text-secondary">{sub}</small> : null}
    </div>
  </div>
);
