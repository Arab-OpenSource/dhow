import { durationOf, secondsBetween } from '@/Modules/Jobs/Types/Duration';
import { isKnownReason } from '@/Modules/Jobs/Types/JobStates';
import { useT } from '@/I18n/useT';

export interface IJobText {
  reason: (code: string) => string;
  duration: (from: string, to: string | number) => string;
}

export const useJobText = (): IJobText => {
  const t = useT();
  return {
    reason: (code) => (isKnownReason(code) ? t(`jobs.reason.${code}`) : code),
    duration: (from, to) => {
      const seconds = secondsBetween(from, to);
      if (seconds === null) return '';
      const value = durationOf(seconds);
      return t(value.Key, value.Params);
    },
  };
};
