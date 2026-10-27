import type { Ar } from '@/I18n/Locales/Ar';

export type TMessageKey = keyof typeof Ar;
export type TMessages = Record<TMessageKey, string>;
export type TMessageParams = Record<string, string | number>;
