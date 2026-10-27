import { z } from 'zod';

import { THEME_NAME_MAX } from '@/Core/Theme/ThemeValidation';

export const themeFormSchema = z.object({
  Name: z
    .string()
    .trim()
    .min(1, { error: 'themeEditor.name.required' })
    .max(THEME_NAME_MAX, { error: 'themeEditor.name.tooLong' }),
});

export type TThemeForm = z.infer<typeof themeFormSchema>;
