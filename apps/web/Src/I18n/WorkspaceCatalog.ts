import type { TLocale } from '@/I18n/Locale';
import type { TWorkspaceMessages } from '@/I18n/Messages';
import { useCatalogStore } from '@/I18n/useCatalogStore';

const LOADERS: Record<TLocale, () => Promise<TWorkspaceMessages>> = {
  ar: () => import('@/I18n/Locales/Workspace/Ar').then((module) => module.WorkspaceAr),
  en: () => import('@/I18n/Locales/Workspace/En').then((module) => module.WorkspaceEn),
};

export const hasWorkspaceMessages = (locale: TLocale): boolean =>
  useCatalogStore.getState().Workspace[locale] !== undefined;

export const anyWorkspaceMessages = (): boolean => Object.keys(useCatalogStore.getState().Workspace).length > 0;

export const loadWorkspaceMessages = async (locale: TLocale): Promise<void> => {
  if (hasWorkspaceMessages(locale)) return;
  const messages = await LOADERS[locale]();
  useCatalogStore.setState((state) => ({ Workspace: { ...state.Workspace, [locale]: messages } }));
};
