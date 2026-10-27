import type { ComponentProps } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import type { TIconName } from '@/Core/Ui/Icons';

export interface IIconButtonProps extends Omit<ComponentProps<'button'>, 'children'> {
  icon: TIconName;
  label: string;
  size?: 'md' | 'lg';
  plain?: boolean;
  badge?: boolean;
}

export const IconButton = ({
  icon,
  label,
  size = 'md',
  plain = false,
  badge = false,
  className,
  type = 'button',
  ...rest
}: IIconButtonProps) => (
  <button
    type={type}
    aria-label={label}
    className={cx(
      'relative inline-grid flex-none cursor-pointer place-items-center rounded-full text-secondary',
      'transition-colors duration-140 hover:text-primary',
      plain ? 'bg-transparent hover:bg-surface-raised' : 'edge',
      size === 'lg' ? 'size-12' : 'size-10',
      className,
    )}
    {...rest}
  >
    <Icon name={icon} size={size === 'lg' ? 20 : 17} />
    {badge ? <i aria-hidden="true" className="absolute end-3 top-[11px] size-1.5 rounded-full bg-accent-danger" /> : null}
  </button>
);
