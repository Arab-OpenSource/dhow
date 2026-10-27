import { Icon } from '@/Core/Ui/Icon';
import type { TIconName } from '@/Core/Ui/Icons';

export interface IIconWellProps {
  icon: TIconName;
}

export const IconWell = ({ icon }: IIconWellProps) => (
  <span
    aria-hidden="true"
    className={
      'inline-grid size-[34px] flex-none place-items-center rounded-full border border-default text-secondary ' +
      'bg-[radial-gradient(circle_at_50%_30%,var(--color-surface-top),var(--color-surface-inset))]'
    }
  >
    <Icon name={icon} size={16} />
  </span>
);
