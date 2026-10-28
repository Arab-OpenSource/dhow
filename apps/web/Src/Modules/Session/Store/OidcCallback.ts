import { useOidcCallbackStore } from '@/Modules/Session/Store/useOidcCallbackStore';

export const OIDC_CALLBACK_PREFIX = '/oidc/callback/';
export const OIDC_CALLBACK_PATH = '/oidc/callback/$provider';
const PROVIDER = /^[a-z][a-z0-9]{0,31}$/;
const MAX_PARAM = 4096;
const PURPOSE_KEY = 'dhow.oidc.purpose';

export type TOidcPurpose = 'login' | 'link';

const param = (params: URLSearchParams, name: string): string => {
  const value = params.get(name) ?? '';
  return value.length > MAX_PARAM ? '' : value;
};

export const providerOf = (pathname: string): string | null => {
  if (!pathname.startsWith(OIDC_CALLBACK_PREFIX)) return null;
  const provider = pathname.slice(OIDC_CALLBACK_PREFIX.length);
  return PROVIDER.test(provider) ? provider : null;
};

export const takeOidcCallbackFromSearch = (pathname: string, search: string): boolean => {
  const provider = providerOf(pathname);
  if (provider === null || search === '' || search === '?') return false;
  const params = new URLSearchParams(search);
  useOidcCallbackStore.getState().Set({
    Provider: provider,
    Code: param(params, 'code'),
    State: param(params, 'state'),
    Error: param(params, 'error'),
  });
  return true;
};

export const takeOidcCallbackFromLocation = (): void => {
  const { pathname, search } = window.location;
  if (takeOidcCallbackFromSearch(pathname, search)) window.history.replaceState(window.history.state, '', pathname);
};

export const rememberOidcPurpose = (purpose: TOidcPurpose): void => {
  try {
    window.sessionStorage.setItem(PURPOSE_KEY, purpose);
  } catch {
    return;
  }
};

export const readOidcPurpose = (): TOidcPurpose => {
  try {
    return window.sessionStorage.getItem(PURPOSE_KEY) === 'link' ? 'link' : 'login';
  } catch {
    return 'login';
  }
};

export const clearOidcPurpose = (): void => {
  try {
    window.sessionStorage.removeItem(PURPOSE_KEY);
  } catch {
    return;
  }
};
