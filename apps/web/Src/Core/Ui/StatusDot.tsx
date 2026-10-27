import { cx } from '@/Core/Ui/ClassNames';

export type TStatus = 'live' | 'building' | 'awaiting' | 'failed' | 'queued' | 'rolledback';

export interface IStatusDotProps {
  status: TStatus;
  children: string;
  className?: string;
}

const DOT: Record<TStatus, string> = {
  live: 'bg-accent-green',
  building: 'bg-accent-blue animate-dhow-pulse',
  awaiting: 'bg-accent-amber',
  failed: 'bg-accent-coral',
  queued: 'bg-accent-violet',
  rolledback: 'bg-track',
};

export const StatusDot = ({ status, children, className }: IStatusDotProps) => (
  <span
    data-status={status}
    className={cx('inline-flex items-center gap-2 whitespace-nowrap text-caption text-secondary', className)}
  >
    <i aria-hidden="true" className={cx('size-2 rounded-full', DOT[status])} />
    {children}
  </span>
);
