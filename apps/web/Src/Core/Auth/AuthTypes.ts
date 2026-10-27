import type { components } from '@/Core/Api/Schema';

type TSchemas = components['schemas'];

export type TRole = TSchemas['Role'];
export type TUser = TSchemas['User'];
export type TMembership = TSchemas['MembershipView'];
export type TOrganization = TSchemas['Organization'];
export type TMe = Omit<TSchemas['Me'], 'CsrfToken'>;

export const ROLES: readonly TRole[] = ['owner', 'admin', 'maintainer', 'developer', 'viewer'];

const AUDIT_ROLES: readonly TRole[] = ['owner', 'admin'];
const ORGANIZATION_EDIT_ROLES: readonly TRole[] = ['owner', 'admin'];
const MEMBER_READ_ROLES: readonly TRole[] = ['owner', 'admin', 'maintainer', 'developer'];

export const canReadAudit = (role: TRole | undefined): boolean => role !== undefined && AUDIT_ROLES.includes(role);

export const canEditOrganization = (role: TRole | undefined): boolean =>
  role !== undefined && ORGANIZATION_EDIT_ROLES.includes(role);

export const canReadMembers = (role: TRole | undefined): boolean => role !== undefined && MEMBER_READ_ROLES.includes(role);
