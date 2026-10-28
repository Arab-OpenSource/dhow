import { BINDABLE_ROLES } from '@/Core/Auth/AuthTypes';
import { emailField, nameField } from '@/Core/Forms/Fields';
import { z } from '@/Core/Forms/Zod';

export const INVITE_FORM_ID = 'organization-invite-form';
export const TEAM_DESCRIPTION_MAX = 500;
export const SLUG_MAX = 63;
export const SLUG_PATTERN = /^[a-z0-9]+(-[a-z0-9]+)*$/;

export const inviteFormSchema = z.object({
  Email: emailField,
  Role: z.enum(BINDABLE_ROLES),
});

export type TInviteForm = z.infer<typeof inviteFormSchema>;
export const INVITE_FIELDS = ['Email', 'Role'] as const satisfies readonly (keyof TInviteForm)[];

export const teamFormSchema = z.object({
  Name: nameField('team.error.name'),
  Description: z.string().trim().max(TEAM_DESCRIPTION_MAX, { error: 'team.error.description' }),
});

export type TTeamForm = z.infer<typeof teamFormSchema>;
export const TEAM_FIELDS = ['Name', 'Description'] as const satisfies readonly (keyof TTeamForm)[];

export const newOrganizationSchema = z.object({
  Name: nameField('setup.error.organizationName'),
  Slug: z
    .string()
    .trim()
    .min(1, { error: 'setup.error.slug' })
    .max(SLUG_MAX, { error: 'setup.error.slug' })
    .regex(SLUG_PATTERN, { error: 'setup.error.slug' }),
});

export type TNewOrganizationForm = z.infer<typeof newOrganizationSchema>;
export const NEW_ORGANIZATION_FIELDS = ['Name', 'Slug'] as const satisfies readonly (keyof TNewOrganizationForm)[];
