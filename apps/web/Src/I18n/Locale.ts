export type TLocale = 'ar' | 'en';
export type TDirection = 'rtl' | 'ltr';

export const LOCALES: readonly TLocale[] = ['ar', 'en'];
export const DEFAULT_LOCALE: TLocale = 'ar';
export const STORAGE_KEY = 'dhow.lang';

export const DIRECTION: Record<TLocale, TDirection> = { ar: 'rtl', en: 'ltr' };

export const isLocale = (value: unknown): value is TLocale => LOCALES.some((locale) => locale === value);

export const readStoredLocale = (): TLocale => {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    return isLocale(stored) ? stored : DEFAULT_LOCALE;
  } catch {
    return DEFAULT_LOCALE;
  }
};

export const storeLocale = (locale: TLocale): void => {
  try {
    window.localStorage.setItem(STORAGE_KEY, locale);
  } catch {
    return;
  }
};

export const otherLocale = (locale: TLocale): TLocale => (locale === 'ar' ? 'en' : 'ar');
