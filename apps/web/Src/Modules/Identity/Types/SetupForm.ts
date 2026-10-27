import { PASSWORD_MAX_LENGTH, PASSWORD_MIN_LENGTH } from '@/Core/Auth/PasswordStrength';
import { z } from '@/Core/Forms/Zod';

export const NAME_MAX = 100;
export const EMAIL_MAX = 254;
export const SLUG_MAX = 63;
export const SLUG_PATTERN = /^[a-z0-9]+(-[a-z0-9]+)*$/;

export const slugify = (name: string): string =>
  name
    .normalize('NFKD')
    .replaceAll(/\p{M}/gu, '')
    .toLowerCase()
    .replaceAll(/[^a-z0-9]+/g, '-')
    .replaceAll(/^-+|-+$/g, '')
    .slice(0, SLUG_MAX)
    .replaceAll(/-+$/g, '');

const name = (required: string) =>
  z.string().trim().min(1, { error: required }).max(NAME_MAX, { error: 'form.error.nameTooLong' });

export const setupFormSchema = z
  .object({
    OwnerName: name('setup.error.ownerName'),
    Email: z
      .string()
      .trim()
      .min(1, { error: 'form.error.emailRequired' })
      .max(EMAIL_MAX, { error: 'form.error.email' })
      .pipe(z.email({ error: 'form.error.email' })),
    Password: z
      .string()
      .min(PASSWORD_MIN_LENGTH, { error: 'password.error.tooShort' })
      .max(PASSWORD_MAX_LENGTH, { error: 'password.error.tooLong' }),
    ConfirmPassword: z.string().min(1, { error: 'password.error.confirmRequired' }),
    OrganizationName: name('setup.error.organizationName'),
    OrganizationSlug: z
      .string()
      .trim()
      .min(1, { error: 'setup.error.slug' })
      .max(SLUG_MAX, { error: 'setup.error.slug' })
      .regex(SLUG_PATTERN, { error: 'setup.error.slug' }),
  })
  .refine((form) => form.Password === form.ConfirmPassword, {
    path: ['ConfirmPassword'],
    error: 'password.error.mismatch',
  });

export type TSetupForm = z.infer<typeof setupFormSchema>;

export const SETUP_FIELDS = [
  'OwnerName',
  'Email',
  'Password',
  'OrganizationName',
  'OrganizationSlug',
] as const satisfies readonly (keyof TSetupForm)[];
