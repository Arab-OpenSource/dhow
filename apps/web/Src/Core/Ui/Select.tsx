import { useContext, type ComponentProps } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { FieldContext } from '@/Core/Ui/FieldContext';

export interface ISelectOption {
  value: string;
  label: string;
}

export interface ISelectProps extends Omit<ComponentProps<'select'>, 'children'> {
  options: readonly ISelectOption[];
  compact?: boolean;
}

export const Select = ({ options, compact = false, className, ...rest }: ISelectProps) => {
  const field = useContext(FieldContext);

  return (
    <select
      id={field?.id}
      aria-describedby={field?.errorId}
      className={cx(
        'w-full min-w-0 cursor-pointer rounded-input border border-line-2 bg-input text-primary',
        compact ? 'h-btn-sm px-2.5 text-body-sm' : 'h-input px-3.5 text-body',
        className,
      )}
      {...rest}
    >
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  );
};
