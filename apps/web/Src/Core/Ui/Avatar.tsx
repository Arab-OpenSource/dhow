import { cx } from '@/Core/Ui/ClassNames';

export type TAvatarSize = 'sm' | 'md' | 'lg';

export interface IAvatarProps {
  name: string;
  size?: TAvatarSize;
  className?: string;
}

const SIZES: Record<TAvatarSize, string> = {
  sm: 'size-8 text-label-sm',
  md: 'size-10 text-label',
  lg: 'size-12 text-title-sm',
};

export const initialOf = (name: string): string => {
  const first = new Intl.Segmenter(undefined, { granularity: 'grapheme' }).segment(name.trim())[Symbol.iterator]().next();
  return first.done ? '' : first.value.segment.toLocaleUpperCase();
};

export const Avatar = ({ name, size = 'md', className }: IAvatarProps) => (
  <span
    aria-hidden="true"
    className={cx('inline-grid flex-none place-items-center rounded-pill bg-surface-raised text-primary', SIZES[size], className)}
  >
    {initialOf(name)}
  </span>
);
