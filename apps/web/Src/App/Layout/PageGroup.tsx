import { useId, type ReactNode } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import type { TIconName } from '@/Core/Ui/Icons';

export interface IPageGroupProps {
  title: string;
  lead?: string;
  icon?: TIconName;
  danger?: boolean;
  children: ReactNode;
}

export const PageGroup = ({ title, lead, icon, danger = false, children }: IPageGroupProps) => {
  const id = useId();
  return (
    <section aria-labelledby={id} data-danger={danger ? '' : undefined} className="grid min-w-0 gap-3">
      <div className="grid gap-1">
        <h2 id={id} className={cx('m-0 flex items-center gap-2 text-title-section', danger && 'text-bad-fg')}>
          {icon ? <Icon name={icon} size={20} className={danger ? 'text-accent-coral' : 'text-secondary'} /> : null}
          {title}
        </h2>
        {lead ? <p className="m-0 text-body-sm text-secondary">{lead}</p> : null}
      </div>
      {children}
    </section>
  );
};
