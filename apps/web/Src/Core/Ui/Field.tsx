import { useId, type ReactNode } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { FieldContext } from '@/Core/Ui/FieldContext';

export interface IFieldProps {
  label: string;
  error?: string;
  className?: string;
  children: ReactNode;
}

export const Field = ({ label, error, className, children }: IFieldProps) => {
  const id = useId();
  const errorId = error ? `${id}-error` : undefined;

  return (
    <FieldContext value={{ id, errorId, invalid: error !== undefined }}>
      <div className={cx('grid gap-1.5', className)}>
        <label htmlFor={id} className="text-body-sm text-secondary">
          {label}
        </label>
        {children}
        {error ? (
          <p id={errorId} role="alert" className="m-0 text-caption text-bad-fg">
            {error}
          </p>
        ) : null}
      </div>
    </FieldContext>
  );
};
