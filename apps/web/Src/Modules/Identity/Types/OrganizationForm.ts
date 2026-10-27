import { nameField } from '@/Core/Forms/Fields';
import { z } from '@/Core/Forms/Zod';

export const organizationFormSchema = z.object({
  Name: nameField('setup.error.organizationName'),
});

export type TOrganizationForm = z.infer<typeof organizationFormSchema>;

export const ORGANIZATION_FIELDS = ['Name'] as const satisfies readonly (keyof TOrganizationForm)[];
