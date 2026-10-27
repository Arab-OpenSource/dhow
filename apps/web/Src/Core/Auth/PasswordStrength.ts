export const PASSWORD_MIN_LENGTH = 12;
export const PASSWORD_MAX_LENGTH = 128;

export type TStrength = 0 | 1 | 2 | 3 | 4;

const MIN_DISTINCT_CHARACTERS = 5;
const GOOD_LENGTH = 16;
const STRONG_LENGTH = 20;

const CLASSES: readonly RegExp[] = [/\p{Ll}/u, /\p{Lu}/u, /\p{Lo}/u, /\p{N}/u, /[^\p{L}\p{N}]/u];

const classesOf = (password: string): number => CLASSES.filter((pattern) => pattern.test(password)).length;

export const scorePassword = (password: string): TStrength => {
  if (password.length === 0) return 0;
  if (password.length < PASSWORD_MIN_LENGTH) return 1;
  if (new Set(password).size < MIN_DISTINCT_CHARACTERS) return 1;
  const classes = classesOf(password);
  if (classes >= 3 && password.length >= STRONG_LENGTH) return 4;
  if (classes >= 2 && password.length >= GOOD_LENGTH) return 3;
  return 2;
};

export const isLengthValid = (password: string): boolean =>
  password.length >= PASSWORD_MIN_LENGTH && password.length <= PASSWORD_MAX_LENGTH;
