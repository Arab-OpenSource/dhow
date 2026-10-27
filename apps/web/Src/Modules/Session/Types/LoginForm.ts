import { emailField } from '@/Core/Forms/Fields';
import { z } from '@/Core/Forms/Zod';

export const loginFormSchema = z.object({
  Email: emailField,
  Password: z.string().min(1, { error: 'login.error.password' }),
});

export type TLoginForm = z.infer<typeof loginFormSchema>;
