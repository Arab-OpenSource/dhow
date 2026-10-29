import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';

export interface ICheckItem<T extends string> {
  value: T;
  title: string;
  about?: string;
  disabled?: boolean;
}

export interface ICheckListProps<T extends string> {
  label: string;
  items: readonly ICheckItem<T>[];
  values: readonly T[];
  onChange: (values: T[]) => void;
  disabled?: boolean;
}

export const CheckList = <T extends string>(props: ICheckListProps<T>) => {
  const { label, items, values, onChange, disabled = false } = props;
  return (
  <div role="group" aria-label={label} className="grid gap-1.5">
    {items.map((item) => {
      const checked = values.includes(item.value);
      return (
        <button
          key={item.value}
          type="button"
          role="checkbox"
          aria-checked={checked}
          disabled={disabled || item.disabled}
          onClick={() => {
            onChange(checked ? values.filter((value) => value !== item.value) : [...values, item.value]);
          }}
          className={cx(
            'flex w-full cursor-pointer items-center gap-3 rounded-md border px-3.5 py-2.5 text-start transition-colors duration-140',
            'disabled:cursor-not-allowed disabled:opacity-60',
            checked ? 'border-line-2 bg-surface-raised' : 'border-line bg-transparent hover:bg-hover',
          )}
        >
          <span
            aria-hidden="true"
            className={cx(
              'grid size-5 flex-none place-items-center rounded-xs border',
              checked ? 'border-accent bg-accent text-on-accent' : 'border-line-2 bg-transparent',
            )}
          >
            {checked ? <Icon name="check" size={14} weight={2.6} /> : null}
          </span>
          <span className="grid min-w-0 flex-1 gap-px">
            <span className="text-label text-primary">{item.title}</span>
            {item.about ? <small className="text-body-sm text-tertiary">{item.about}</small> : null}
          </span>
        </button>
      );
    })}
  </div>
);
};
