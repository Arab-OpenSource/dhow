import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';

export interface IChoice<T extends string> {
  value: T;
  title: string;
  about?: string;
  disabled?: boolean;
}

export interface IChoiceListProps<T extends string> {
  label: string;
  choices: readonly IChoice<T>[];
  value: T | null;
  onChange: (value: T) => void;
  disabled?: boolean;
}

export const ChoiceList = <T extends string>(props: IChoiceListProps<T>) => {
  const { label, choices, value, onChange, disabled = false } = props;
  return (
  <div role="radiogroup" aria-label={label} className="grid gap-1.5">
    {choices.map((choice) => {
      const checked = choice.value === value;
      return (
        <button
          key={choice.value}
          type="button"
          role="radio"
          aria-checked={checked}
          disabled={disabled || choice.disabled}
          onClick={() => {
            onChange(choice.value);
          }}
          className={cx(
            'flex w-full cursor-pointer items-center gap-3 rounded-md border px-3.5 py-2.5 text-start transition-colors duration-140',
            'disabled:cursor-not-allowed disabled:opacity-60',
            checked ? 'border-line-2 bg-surface-raised' : 'border-line bg-transparent hover:bg-hover',
          )}
        >
          <span className="grid min-w-0 flex-1 gap-px">
            <span className="text-label text-primary">{choice.title}</span>
            {choice.about ? <small className="text-body-sm text-tertiary">{choice.about}</small> : null}
          </span>
          {checked ? <Icon name="check" size={16} weight={2.4} className="flex-none text-primary" /> : null}
        </button>
      );
    })}
  </div>
);
};
