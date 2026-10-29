import { Ar } from '@/I18n/Locales/Ar';
import type { TMessageKey, TMessageParams } from '@/I18n/Messages';
import { translate } from '@/I18n/Translate';

export const arText = (key: TMessageKey, params?: TMessageParams): string => translate('ar', key, params, Ar);
