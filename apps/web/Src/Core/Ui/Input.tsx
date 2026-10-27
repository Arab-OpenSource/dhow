import { useContext, type ComponentProps } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { FieldContext } from '@/Core/Ui/FieldContext';

export interface IInputProps extends ComponentProps<'input'> {
  compact?: boolean;
  invalid?: boolean;
}

export const Input = ({ compact = false, invalid, className, type = 'text', ...rest }: IInputProps) => {
  const field = useContext(FieldContext);
  const bad = invalid ?? field?.invalid ?? false;

  return (
    <input
      id={field?.id}
      type={type}
      aria-invalid={bad ? true : undefined}
      aria-describedby={field?.errorId}
      className={cx(
        'w-full min-w-0 rounded-input border border-line-2 bg-input text-primary placeholder:text-tertiary',
        'aria-invalid:border-accent-coral',
        compact ? 'h-btn-sm px-2.5 text-caption' : 'h-input px-3.5 text-body',
        className,
      )}
      {...rest}
    />
  );
};
