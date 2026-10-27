import { useContext } from 'react';

import { I18nContext, type II18nValue } from '@/I18n/I18nContext';

export const useI18n = (): II18nValue => {
  const value = useContext(I18nContext);
  if (!value) throw new Error('useI18n must be used inside I18nProvider');
  return value;
};
