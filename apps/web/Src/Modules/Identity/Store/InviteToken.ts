import { tokenFromHash } from '@/Modules/Identity/Store/SetupToken';
import { useInviteTokenStore } from '@/Modules/Identity/Store/useInviteTokenStore';

export const INVITE_PATH = '/invite';

export const takeInviteTokenFromLocation = (): void => {
  const { pathname, search, hash } = window.location;
  if (pathname !== INVITE_PATH || hash === '') return;
  const token = tokenFromHash(hash);
  if (token !== null) useInviteTokenStore.getState().Set(token);
  window.history.replaceState(window.history.state, '', `${pathname}${search}`);
};

export const takeInviteTokenFromHash = (hash: string): boolean => {
  if (hash === '') return false;
  const token = tokenFromHash(hash);
  if (token !== null) useInviteTokenStore.getState().Set(token);
  return true;
};
