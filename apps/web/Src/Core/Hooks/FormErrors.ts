import type { FieldValues, Path, UseFormReturn } from 'react-hook-form';

import type { ApiError } from '@/Core/Api/ApiError';

export const SERVER_ERROR_TYPE = 'server';

export const applyFieldErrors = <TForm extends FieldValues>(
  form: UseFormReturn<TForm>,
  error: ApiError,
  fields: readonly Path<TForm>[],
): boolean => {
  const known = error.Details.flatMap((detail) => {
    const field = fields.find((name) => name === detail.Field);
    return field === undefined ? [] : [{ field, message: detail.Message }];
  });
  for (const { field, message } of known) {
    form.setError(field, { type: SERVER_ERROR_TYPE, message });
  }
  const [first] = known;
  if (first) form.setFocus(first.field);
  return known.length > 0;
};
