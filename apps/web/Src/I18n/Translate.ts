import type { TLocale } from '@/I18n/Locale';
import { BootAr } from '@/I18n/Locales/Boot/Ar';
import { BootEn } from '@/I18n/Locales/Boot/En';
import type { TMessageKey, TMessageParams, TMessages, TWorkspaceMessages } from '@/I18n/Messages';
import { useCatalogStore } from '@/I18n/useCatalogStore';

type TCatalog = Readonly<Partial<TMessages>>;

const BOOT: Record<TLocale, TCatalog> = { ar: BootAr, en: BootEn };

const loadedWorkspace = (locale: TLocale): TWorkspaceMessages | undefined =>
  useCatalogStore.getState().Workspace[locale];

export const translate = (
  locale: TLocale,
  key: TMessageKey,
  params?: TMessageParams,
  workspace: TCatalog | undefined = loadedWorkspace(locale),
): string => {
  const template = BOOT[locale][key] ?? workspace?.[key] ?? key;
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (placeholder, name: string) => {
    const value = params[name];
    return value === undefined ? placeholder : String(value);
  });
};

export const isMessageKey = (key: string): key is TMessageKey =>
  Object.hasOwn(BootAr, key) ||
  Object.values(useCatalogStore.getState().Workspace).some((messages) => Object.hasOwn(messages, key));
