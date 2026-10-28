import { canManageRunners, canReadRunners } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership, useMe } from '@/Core/Auth/useMe';

export type TRunnerAccess =
  | { Kind: 'loading' }
  | { Kind: 'denied' }
  | { Kind: 'allowed'; OrganizationId: string; Manager: boolean };

export const useRunnerAccess = (): TRunnerAccess => {
  const me = useMe();
  const membership = useCurrentMembership();
  if (membership === undefined) return me.isPending ? { Kind: 'loading' } : { Kind: 'denied' };
  if (!canReadRunners(membership.Role)) return { Kind: 'denied' };
  return { Kind: 'allowed', OrganizationId: membership.OrganizationId, Manager: canManageRunners(membership.Role) };
};
