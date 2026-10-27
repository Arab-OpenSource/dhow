import type { TMfaMethod } from '@/Core/Auth/AuthApi';

export type TCodeMethod = Exclude<TMfaMethod, 'passkey'>;

const TOTP_PATTERN = /^\d{6}$/;
const RECOVERY_PATTERN = /^[0-9A-Za-z]{16}$/;

export const cleanCode = (method: TCodeMethod, value: string): string | null => {
  const compact = value.replace(/[\s-]/g, '');
  const pattern = method === 'totp' ? TOTP_PATTERN : RECOVERY_PATTERN;
  return pattern.test(compact) ? compact : null;
};

export const firstCodeMethod = (methods: readonly TMfaMethod[]): TCodeMethod | null => {
  if (methods.includes('totp')) return 'totp';
  if (methods.includes('recovery_code')) return 'recovery_code';
  return null;
};
