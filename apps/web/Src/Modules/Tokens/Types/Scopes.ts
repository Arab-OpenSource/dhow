import type { TRole } from '@/Core/Auth/AuthTypes';
import type { TApiTokenScope } from '@/Modules/Tokens/Api/TokensApi';

export const SCOPES: readonly TApiTokenScope[] = [
  'org.read',
  'org.update',
  'member.read',
  'member.manage',
  'audit.read',
  'runner.read',
  'runner.manage',
];

const READ_ONLY: readonly TApiTokenScope[] = ['org.read', 'member.read', 'runner.read'];

export const scopesFor = (role: TRole): readonly TApiTokenScope[] => {
  switch (role) {
    case 'owner':
    case 'admin':
      return SCOPES;
    case 'maintainer':
    case 'developer':
      return READ_ONLY;
    case 'viewer':
      return ['org.read'];
    default:
      return [];
  }
};

export const EXPIRY_DAYS = [7, 30, 90, 365] as const;
export type TExpiry = `${(typeof EXPIRY_DAYS)[number]}`;
export const DEFAULT_EXPIRY_DAYS = 30;
export const TOKEN_NAME_MAX = 100;
