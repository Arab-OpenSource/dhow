import { create } from 'zustand';

import type { TLocale } from '@/I18n/Locale';
import type { TWorkspaceMessages } from '@/I18n/Messages';

interface ICatalogState {
  Workspace: Partial<Record<TLocale, TWorkspaceMessages>>;
}

export const useCatalogStore = create<ICatalogState>(() => ({ Workspace: {} }));
