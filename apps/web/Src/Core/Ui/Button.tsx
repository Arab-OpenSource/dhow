import type { ComponentProps } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import type { TIconName } from '@/Core/Ui/Icons';

export type TButtonVariant = 'edge' | 'aurora' | 'quiet';

export interface IButtonProps extends ComponentProps<'button'> {
  variant?: TButtonVariant;
  size?: 'sm' | 'md';
  iconStart?: TIconName;
}

const VARIANTS: Record<TButtonVariant, string> = {
  edge: 'edge text-primary hover:[--edge-fill:var(--color-surface-raised)]',
  aurora: 'bg-aurora text-on-light',
  quiet: 'bg-transparent text-secondary hover:bg-surface-raised hover:text-primary',
};

export const Button = ({
  variant = 'edge',
  size = 'md',
  iconStart,
  className,
  children,
  type = 'button',
  ...rest
}: IButtonProps) => (
  <button
    type={type}
    className={cx(
      'inline-flex cursor-pointer items-center justify-center gap-2 whitespace-nowrap rounded-pill font-medium',
      'transition-[background-color,color,transform] duration-140 ease-dhow active:scale-[0.98]',
      'disabled:cursor-not-allowed disabled:border disabled:border-subtle disabled:text-muted',
      'disabled:[background:none] disabled:active:scale-100',
      size === 'sm' ? 'h-[34px] px-4 text-[13px]' : 'h-10 px-[18px] text-[14px]',
      VARIANTS[variant],
      className,
    )}
    {...rest}
  >
    {iconStart ? <Icon name={iconStart} size={16} /> : null}
    {children}
  </button>
);
