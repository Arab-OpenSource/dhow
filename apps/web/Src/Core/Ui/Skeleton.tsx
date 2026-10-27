import { cx } from '@/Core/Ui/ClassNames';

export interface ISkeletonProps {
  label: string;
  lines?: number;
  className?: string;
}

export const Skeleton = ({ label, lines = 3, className }: ISkeletonProps) => (
  <div role="status" aria-busy="true" aria-label={label} className={cx('grid w-full gap-2.5', className)}>
    {Array.from({ length: lines }, (_, index) => (
      <i
        key={index}
        aria-hidden="true"
        className="block h-3 animate-dhow-fade rounded-pill bg-surface-raised"
        style={{ width: `${index === lines - 1 ? 55 : 100 - index * 9}%` }}
      />
    ))}
  </div>
);
