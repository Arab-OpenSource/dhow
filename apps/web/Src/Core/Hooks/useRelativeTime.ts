import { agoOf, formatDateTime } from '@/Core/Format/TimeFormat';
import { useI18n } from '@/I18n/useI18n';

export interface ITimeFormatter {
  ago: (iso: string) => string;
  full: (iso: string) => string;
}

export const useRelativeTime = (): ITimeFormatter => {
  const { Locale, t } = useI18n();

  return {
    ago: (iso) => {
      const ago = agoOf(iso, Date.now());
      if (ago === null) return formatDateTime(iso, Locale);
      return ago.Count === undefined ? t(ago.Key) : t(ago.Key, { count: ago.Count });
    },
    full: (iso) => formatDateTime(iso, Locale),
  };
};
