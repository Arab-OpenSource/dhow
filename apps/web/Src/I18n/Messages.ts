import type { Ar } from '@/I18n/Locales/Ar';
import type { BootAr } from '@/I18n/Locales/Boot/Ar';
import type { WorkspaceAr } from '@/I18n/Locales/Workspace/Ar';

export type TMessageKey = keyof typeof Ar;
export type TMessages = Record<TMessageKey, string>;
export type TBootMessages = Record<keyof typeof BootAr, string>;
export type TWorkspaceMessages = Record<keyof typeof WorkspaceAr, string>;
export type TMessageParams = Record<string, string | number>;
