import type { TLocale } from '@/I18n/Locale';
import type { TMessageKey } from '@/I18n/Messages';

const SECONDS_PER_MINUTE = 60;
const MINUTES_PER_HOUR = 60;
const HOURS_PER_DAY = 24;
const JUST_NOW_SECONDS = 45;
const DAYS_BEFORE_DATE = 30;

export interface IAgo {
  Key: TMessageKey;
  Count?: number;
}

export const formatClock = (totalSeconds: number): string => {
  const safe = Math.max(0, Math.floor(totalSeconds));
  const minutes = Math.floor(safe / SECONDS_PER_MINUTE);
  const seconds = safe % SECONDS_PER_MINUTE;
  return `${minutes}:${String(seconds).padStart(2, '0')}`;
};

export const agoOf = (iso: string, nowMs: number): IAgo | null => {
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return null;
  const seconds = Math.max(0, Math.floor((nowMs - then) / 1000));
  if (seconds < JUST_NOW_SECONDS) return { Key: 'time.now' };
  const minutes = Math.round(seconds / SECONDS_PER_MINUTE);
  if (minutes < MINUTES_PER_HOUR) return minutes === 1 ? { Key: 'time.minute.one' } : { Key: 'time.minutes', Count: minutes };
  const hours = Math.round(minutes / MINUTES_PER_HOUR);
  if (hours < HOURS_PER_DAY) return hours === 1 ? { Key: 'time.hour.one' } : { Key: 'time.hours', Count: hours };
  const days = Math.round(hours / HOURS_PER_DAY);
  if (days > DAYS_BEFORE_DATE) return null;
  return days === 1 ? { Key: 'time.day.one' } : { Key: 'time.days', Count: days };
};

const INTL_LOCALES: Record<TLocale, string> = { ar: 'ar-u-nu-latn', en: 'en' };

export const formatDateTime = (iso: string, locale: TLocale): string => {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '';
  return new Intl.DateTimeFormat(INTL_LOCALES[locale], { dateStyle: 'medium', timeStyle: 'short' }).format(date);
};
