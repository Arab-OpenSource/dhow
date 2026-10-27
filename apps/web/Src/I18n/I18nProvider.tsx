import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';

import { I18nContext, type II18nValue } from '@/I18n/I18nContext';
import { DIRECTION, readStoredLocale, storeLocale, type TLocale } from '@/I18n/Locale';
import { translate } from '@/I18n/Translate';

export const I18nProvider = ({ children }: { children: ReactNode }) => {
  const [Locale, setLocaleState] = useState<TLocale>(readStoredLocale);

  const t = useCallback<II18nValue['t']>((key, params) => translate(Locale, key, params), [Locale]);

  const SetLocale = useCallback((next: TLocale) => {
    storeLocale(next);
    setLocaleState(next);
  }, []);

  useEffect(() => {
    const root = document.documentElement;
    root.lang = Locale;
    root.dir = DIRECTION[Locale];
    document.title = translate(Locale, 'app.title');
  }, [Locale]);

  const value = useMemo<II18nValue>(() => ({ Locale, SetLocale, t }), [Locale, SetLocale, t]);

  return <I18nContext value={value}>{children}</I18nContext>;
};
