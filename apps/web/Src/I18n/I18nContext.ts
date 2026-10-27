import { createContext } from 'react';

import type { TLocale } from '@/I18n/Locale';
import type { TMessageKey, TMessageParams } from '@/I18n/Messages';

export type TTranslate = (key: TMessageKey, params?: TMessageParams) => string;

export interface II18nValue {
  Locale: TLocale;
  SetLocale: (locale: TLocale) => void;
  t: TTranslate;
}

export const I18nContext = createContext<II18nValue | null>(null);
