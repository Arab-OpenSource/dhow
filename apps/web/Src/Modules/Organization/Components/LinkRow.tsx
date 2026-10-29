import type { ReactNode } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import { followInApp } from '@/Core/Ui/PlainClick';

export interface ILinkRowProps {
  href: string;
  current: boolean;
  title: ReactNode;
  sub?: ReactNode;
  start?: ReactNode;
  end?: ReactNode;
  onSelect: () => void;
}

export const LinkRow = ({ href, current, title, sub, start, end, onSelect }: ILinkRowProps) => (
  <li className="min-w-0 border-t border-line first:border-t-0">
    <a
      href={href}
      aria-current={current ? 'page' : undefined}
      onClick={followInApp(onSelect)}
      className={cx(
        'flex min-w-0 items-center gap-3 px-card py-3 text-primary no-underline transition-colors duration-140 [li:last-child_&]:rounded-b-md',
        current ? 'bg-sel' : 'hover:bg-surface-raised',
      )}
    >
      {start}
      <span className="grid min-w-0 flex-1 gap-0.5">
        <span className="min-w-0 truncate text-label">{title}</span>
        {sub ? <small className="min-w-0 text-body-sm text-tertiary">{sub}</small> : null}
      </span>
      {end ? <span className="flex flex-none items-center gap-1.5">{end}</span> : null}
      <Icon name="next" size={16} className="flex-none text-tertiary" />
    </a>
  </li>
);
