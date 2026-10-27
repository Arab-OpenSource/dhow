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
      'grid min-w-0 cursor-pointer gap-2 rounded-lg border border-subtle bg-row p-2 text-start text-primary',
      'hover:border-strong',
      active && 'edge [--edge-fill:var(--color-surface-raised)]',
    )}
  >
    <span data-theme={id} aria-hidden="true" className="relative block h-16 overflow-hidden rounded-[10px] bg-canvas">
      <i className="absolute start-[34%] end-2.5 top-2.5 bottom-[22px] rounded-[6px] bg-surface" />
      <i className="absolute start-2.5 top-2.5 size-3.5 rounded-full bg-aurora-1" />
      <i
        className={
          'absolute inset-x-2.5 bottom-2.5 h-1.5 rounded-[6px] ' +
          'bg-[linear-gradient(var(--gradient-angle),var(--color-aurora-1),var(--color-aurora-3),var(--color-aurora-5))]'
        }
      />
    </span>
    <span className="flex items-center gap-1.5 text-caption">
      {name}
      {active ? <Icon name="check" size={14} weight={2.4} className="ms-auto text-accent-green" /> : null}
    </span>
  </button>
);
