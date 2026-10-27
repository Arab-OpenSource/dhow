import { useSetupTokenStore } from '@/Modules/Identity/Store/useSetupTokenStore';

export const SETUP_PATH = '/setup';
const TOKEN_PATTERN = /^[A-Za-z0-9_-]{16,128}$/;

export const tokenFromHash = (hash: string): string | null => {
  const raw = hash.startsWith('#') ? hash.slice(1) : hash;
  return TOKEN_PATTERN.test(raw) ? raw : null;
};

export const takeSetupTokenFromLocation = (): void => {
  const { pathname, search, hash } = window.location;
  if (pathname !== SETUP_PATH || hash === '') return;
  const token = tokenFromHash(hash);
  if (token !== null) useSetupTokenStore.getState().Set(token);
  window.history.replaceState(window.history.state, '', `${pathname}${search}`);
};

export const takeSetupTokenFromHash = (hash: string): boolean => {
  if (hash === '') return false;
  const token = tokenFromHash(hash);
  if (token !== null) useSetupTokenStore.getState().Set(token);
  return true;
};
