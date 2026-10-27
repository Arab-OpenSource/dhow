import { PASSWORD_MAX_LENGTH, PASSWORD_MIN_LENGTH } from '@/Core/Auth/PasswordStrength';
import { z } from '@/Core/Forms/Zod';

export const PASSWORD_FORM_ID = 'change-password-form';

export const passwordFormSchema = z
  .object({
    CurrentPassword: z.string().min(1, { error: 'password.error.currentRequired' }),
    NewPassword: z
      .string()
      .min(PASSWORD_MIN_LENGTH, { error: 'password.error.tooShort' })
      .max(PASSWORD_MAX_LENGTH, { error: 'password.error.tooLong' }),
    ConfirmPassword: z.string().min(1, { error: 'password.error.confirmRequired' }),
  })
  .refine((form) => form.NewPassword === form.ConfirmPassword, {
    path: ['ConfirmPassword'],
    error: 'password.error.mismatch',
  });

export type TPasswordForm = z.infer<typeof passwordFormSchema>;

export const PASSWORD_FIELDS = ['CurrentPassword', 'NewPassword'] as const satisfies readonly (keyof TPasswordForm)[];
