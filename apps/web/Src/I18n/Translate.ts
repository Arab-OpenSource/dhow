import { Ar } from '@/I18n/Locales/Ar';
import { En } from '@/I18n/Locales/En';
import type { TLocale } from '@/I18n/Locale';
import type { TMessageKey, TMessageParams, TMessages } from '@/I18n/Messages';

const CATALOGS: Record<TLocale, TMessages> = { ar: Ar, en: En };

export const translate = (locale: TLocale, key: TMessageKey, params?: TMessageParams): string => {
  const template = CATALOGS[locale][key];
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (placeholder, name: string) => {
    const value = params[name];
    return value === undefined ? placeholder : String(value);
  });
};

export const catalogOf = (locale: TLocale): TMessages => CATALOGS[locale];
