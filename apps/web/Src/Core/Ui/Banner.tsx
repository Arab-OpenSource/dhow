import type { ReactNode } from 'react';

import { ACCENT_ICON, ACCENT_SURFACE, ACCENT_TEXT, type TAccent } from '@/Core/Ui/Accent';
import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import { IconButton } from '@/Core/Ui/IconButton';

export interface IBannerProps {
  accent?: TAccent;
  title: string;
  body?: string;
  action?: ReactNode;
  dismissLabel?: string;
  onDismiss?: () => void;
  wrap?: boolean;
  className?: string;
}

export const Banner = ({
  accent = 'awaiting',
  title,
  body,
  action,
  dismissLabel,
  onDismiss,
  wrap = false,
  className,
}: IBannerProps) => (
  <div
    role={accent === 'failed' ? 'alert' : 'status'}
    data-accent={accent}
    className={cx(
      'box-border flex w-full items-center gap-3 rounded-md border px-4 py-2.5 text-body-sm',
      wrap && 'flex-wrap',
      ACCENT_SURFACE[accent],
      className,
    )}
  >
    <Icon name={ACCENT_ICON[accent]} size={18} className={ACCENT_TEXT[accent]} />
    <span className={cx('flex flex-1 flex-wrap gap-x-2 gap-y-0.5', wrap && 'min-w-[min(100%-2.5rem,22rem)]')}>
      {title}
      {body ? <span className="text-secondary">{body}</span> : null}
    </span>
    {action}
    {onDismiss && dismissLabel ? <IconButton icon="close" label={dismissLabel} plain onClick={onDismiss} /> : null}
  </div>
);
