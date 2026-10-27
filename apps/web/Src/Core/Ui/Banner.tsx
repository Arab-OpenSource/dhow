import type { ReactNode } from 'react';

import { ACCENT_ICON, ACCENT_TEXT, type TAccent } from '@/Core/Ui/Accent';
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
  className?: string;
}

export const Banner = ({
  accent = 'awaiting',
  title,
  body,
  action,
  dismissLabel,
  onDismiss,
  className,
}: IBannerProps) => (
  <div
    role={accent === 'failed' ? 'alert' : 'status'}
    data-accent={accent}
    className={cx(
      'box-border flex w-full items-center gap-3 rounded-md border border-subtle bg-surface-inset px-4 py-2.5 text-[14px]',
      className,
    )}
  >
    <Icon name={ACCENT_ICON[accent]} size={18} className={ACCENT_TEXT[accent]} />
    <span className="flex flex-1 flex-wrap gap-x-2 gap-y-0.5">
      {title}
      {body ? <span className="text-tertiary">{body}</span> : null}
    </span>
    {action}
    {onDismiss && dismissLabel ? <IconButton icon="close" label={dismissLabel} plain onClick={onDismiss} /> : null}
  </div>
);
