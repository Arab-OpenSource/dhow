import { z } from '@/Core/Forms/Zod';
import type { TMessageKey } from '@/I18n/Messages';

export const NAME_MAX = 100;
export const EMAIL_MAX = 254;

export const emailField = z
  .string()
  .trim()
  .min(1, { error: 'form.error.emailRequired' })
  .max(EMAIL_MAX, { error: 'form.error.email' })
  .pipe(z.email({ error: 'form.error.email' }));

export const nameField = (required: TMessageKey) =>
  z.string().trim().min(1, { error: required }).max(NAME_MAX, { error: 'form.error.nameTooLong' });
