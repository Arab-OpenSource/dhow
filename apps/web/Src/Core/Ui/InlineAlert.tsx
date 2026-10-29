import type { ReactNode } from 'react';

import { ACCENT_ICON, ACCENT_SURFACE, ACCENT_TEXT, type TAccent } from '@/Core/Ui/Accent';
import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';

export interface IInlineAlertProps {
  accent?: TAccent;
  title: string;
  children?: ReactNode;
  className?: string;
}

export const InlineAlert = ({ accent = 'building', title, children, className }: IInlineAlertProps) => (
  <div
    role={accent === 'failed' ? 'alert' : 'note'}
    data-accent={accent}
    className={cx('box-border flex w-full gap-3 rounded-md border px-4 py-3 text-body-sm', ACCENT_SURFACE[accent], className)}
  >
    <Icon name={ACCENT_ICON[accent]} size={17} className={cx('mt-[3px]', ACCENT_TEXT[accent])} />
    <span className="grid gap-0.5">
      {title}
      {children ? <span className="text-body-sm text-secondary">{children}</span> : null}
    </span>
  </div>
);
