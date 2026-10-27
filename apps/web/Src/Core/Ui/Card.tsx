import type { ReactNode } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import type { TIconName } from '@/Core/Ui/Icons';

export interface ICardProps {
  title?: string;
  icon?: TIconName;
  end?: ReactNode;
  padded?: boolean;
  className?: string;
  children: ReactNode;
}

export const Card = ({ title, icon, end, padded = false, className, children }: ICardProps) => (
  <section className={cx('min-w-0 rounded-md border border-line bg-surface', className)}>
    {title ? (
      <header className="flex items-center gap-2.5 px-4 pt-3.5 pb-1.5">
        {icon ? <Icon name={icon} size={18} className="text-secondary" /> : null}
        <h3 className="m-0 text-title-sm">{title}</h3>
        {end ? <span className="ms-auto text-caption text-tertiary">{end}</span> : null}
      </header>
    ) : null}
    <div className={cx('grid', padded && (title ? 'px-4 pt-1.5 pb-4' : 'p-4'))}>{children}</div>
  </section>
);
