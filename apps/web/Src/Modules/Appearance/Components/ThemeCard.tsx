import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import type { TSystemThemeId } from '@/Core/Theme/ThemeTokens';

export interface IThemeCardProps {
  id: TSystemThemeId;
  name: string;
  active: boolean;
  onUse: () => void;
}

export const ThemeCard = ({ id, name, active, onUse }: IThemeCardProps) => (
  <button
    type="button"
    aria-pressed={active}
    onClick={onUse}
    className={cx(
      'grid min-w-0 cursor-pointer gap-2 rounded-md border p-2 text-start text-primary transition-colors duration-140',
      active ? 'border-accent bg-sel' : 'border-line bg-surface-inset hover:border-line-2',
    )}
  >
    <span data-theme={id} aria-hidden="true" className="relative block h-16 overflow-hidden rounded-sm bg-canvas">
      <i className="absolute start-[34%] end-2.5 top-2.5 bottom-[22px] rounded-xs bg-surface" />
      <i className="absolute start-2.5 top-2.5 size-3.5 rounded-pill bg-accent" />
      <i className="absolute inset-x-2.5 bottom-2.5 h-1.5 rounded-xs bg-accent" />
    </span>
    <span className="flex items-center gap-1.5 text-body-sm">
      {name}
      {active ? <Icon name="check" size={14} weight={2.4} className="ms-auto text-accent" /> : null}
    </span>
  </button>
);
