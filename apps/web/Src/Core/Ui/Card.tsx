import type { ReactNode } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { IconWell } from '@/Core/Ui/IconWell';
import type { TIconName } from '@/Core/Ui/Icons';

export interface ICardProps {
  title?: string;
  icon?: TIconName;
  end?: ReactNode;
  className?: string;
  children: ReactNode;
}

export const Card = ({ title, icon, end, className, children }: ICardProps) => (
  <section className={cx('relative rounded-lg bg-card-gradient p-4 shadow-card', className)}>
    {title ? (
      <header className="mb-3 flex items-center gap-2.5">
        {icon ? <IconWell icon={icon} /> : null}
        <h3 className="m-0 text-title-lg">{title}</h3>
        {end ? <span className="ms-auto text-caption text-tertiary">{end}</span> : null}
      </header>
    ) : null}
    {children}
  </section>
);
