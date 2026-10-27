const ENTRY_PATHS = ['/login', '/setup'];

export const safeRedirect = (value: unknown): string | null => {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//') || value.includes('\\')) return null;
  const path = value.split(/[?#]/)[0] ?? '';
  return ENTRY_PATHS.includes(path) ? null : value;
};
