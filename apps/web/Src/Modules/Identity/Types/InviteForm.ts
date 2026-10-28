import { PASSWORD_MAX_LENGTH, PASSWORD_MIN_LENGTH } from '@/Core/Auth/PasswordStrength';
import { nameField } from '@/Core/Forms/Fields';
import { z } from '@/Core/Forms/Zod';

export const inviteAcceptSchema = z.object({
  Name: nameField('invite.page.error.name'),
  Password: z
    .string()
    .min(PASSWORD_MIN_LENGTH, { error: 'password.error.tooShort' })
    .max(PASSWORD_MAX_LENGTH, { error: 'password.error.tooLong' }),
});

export type TInviteAcceptForm = z.infer<typeof inviteAcceptSchema>;

export const INVITE_ACCEPT_FIELDS = ['Name', 'Password'] as const satisfies readonly (keyof TInviteAcceptForm)[];
