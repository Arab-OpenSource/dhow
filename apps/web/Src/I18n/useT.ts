import type { TTranslate } from '@/I18n/I18nContext';
import { useI18n } from '@/I18n/useI18n';

export const useT = (): TTranslate => useI18n().t;
