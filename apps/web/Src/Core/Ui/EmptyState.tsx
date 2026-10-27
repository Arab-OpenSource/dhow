import type { ReactNode } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { IconWell } from '@/Core/Ui/IconWell';
import type { TIconName } from '@/Core/Ui/Icons';

export interface IEmptyStateProps {
  icon: TIconName;
  text: string;
  action?: ReactNode;
  className?: string;
}

export const EmptyState = ({ icon, text, action, className }: IEmptyStateProps) => (
  <div className={cx('grid justify-items-center gap-3 px-5 py-8 text-center', className)}>
    <IconWell icon={icon} />
    <p className="m-0 max-w-[40ch] text-caption text-tertiary">{text}</p>
    {action}
  </div>
);
