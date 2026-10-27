export type TTone = 'ok' | 'warn' | 'bad';

export const TONE_SURFACE: Record<TTone, string> = {
  ok: 'border-ok-line bg-ok-wash',
  warn: 'border-warn-line bg-warn-wash',
  bad: 'border-bad-line bg-bad-wash',
};

export const TONE_TEXT: Record<TTone, string> = {
  ok: 'text-ok-fg',
  warn: 'text-warn-fg',
  bad: 'text-bad-fg',
};
