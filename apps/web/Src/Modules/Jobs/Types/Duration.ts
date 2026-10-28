import type { TMessageKey, TMessageParams } from '@/I18n/Messages';

export interface IDuration {
  Key: TMessageKey;
  Params: TMessageParams;
}

const MINUTE = 60;
const HOUR = 3600;

export const secondsBetween = (from: string, to: string | number): number | null => {
  const start = Date.parse(from);
  const end = typeof to === 'number' ? to : Date.parse(to);
  if (Number.isNaN(start) || Number.isNaN(end)) return null;
  return Math.max(0, Math.floor((end - start) / 1000));
};

export const durationOf = (seconds: number): IDuration => {
  if (seconds < MINUTE) return { Key: 'jobs.duration.seconds', Params: { s: seconds } };
  if (seconds < HOUR) {
    return { Key: 'jobs.duration.minutes', Params: { m: Math.floor(seconds / MINUTE), s: seconds % MINUTE } };
  }
  return { Key: 'jobs.duration.hours', Params: { h: Math.floor(seconds / HOUR), m: Math.floor((seconds % HOUR) / MINUTE) } };
};

export const formatBytes = (bytes: number): { Key: TMessageKey; Params: TMessageParams } => {
  const gib = 1024 ** 3;
  const mib = 1024 ** 2;
  if (bytes >= gib && bytes % gib === 0) return { Key: 'jobs.size.gib', Params: { n: bytes / gib } };
  if (bytes >= gib) return { Key: 'jobs.size.gib', Params: { n: (bytes / gib).toFixed(1) } };
  if (bytes >= mib) return { Key: 'jobs.size.mib', Params: { n: Math.round(bytes / mib) } };
  return { Key: 'jobs.size.bytes', Params: { n: bytes } };
};
