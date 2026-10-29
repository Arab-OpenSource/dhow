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
      <header className="flex items-center gap-2.5 px-card pt-4 pb-2">
        {icon ? <Icon name={icon} size={18} className="text-secondary" /> : null}
        <h3 className="m-0 text-title-card">{title}</h3>
        {end ? <span className="ms-auto text-body-sm text-tertiary">{end}</span> : null}
      </header>
    ) : null}
    <div className={cx('grid', padded && (title ? 'px-card pt-1 pb-card' : 'p-card'))}>{children}</div>
  </section>
);
