import { ICONS, MIRRORED_ICONS, type TIconName } from '@/Core/Ui/Icons';
import { cx } from '@/Core/Ui/ClassNames';

export interface IIconProps {
  name: TIconName;
  size?: number;
  weight?: number;
  label?: string;
  className?: string;
}

export const Icon = ({ name, size = 18, weight = 1.6, label, className }: IIconProps) => (
  <svg
    viewBox="0 0 24 24"
    width={size}
    height={size}
    fill="none"
    stroke="currentColor"
    strokeWidth={weight}
    strokeLinecap="round"
    strokeLinejoin="round"
    focusable="false"
    role={label ? 'img' : undefined}
    aria-label={label}
    aria-hidden={label ? undefined : true}
    className={cx(
      'inline-block flex-none align-middle',
      MIRRORED_ICONS.includes(name) && 'rtl:-scale-x-100',
      className,
    )}
  >
    {ICONS[name]}
  </svg>
);
