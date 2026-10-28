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
  'job.read',
  'job.cancel',
];

const DEVELOPER: readonly TApiTokenScope[] = ['org.read', 'member.read', 'runner.read', 'job.read'];
const MAINTAINER: readonly TApiTokenScope[] = [...DEVELOPER, 'job.cancel'];

export const scopesFor = (role: TRole): readonly TApiTokenScope[] => {
  switch (role) {
    case 'owner':
    case 'admin':
      return SCOPES;
    case 'maintainer':
      return MAINTAINER;
    case 'developer':
      return DEVELOPER;
    case 'viewer':
      return ['org.read', 'job.read'];
    default:
      return [];
  }
};

export const EXPIRY_DAYS = [7, 30, 90, 365] as const;
export type TExpiry = `${(typeof EXPIRY_DAYS)[number]}`;
export const DEFAULT_EXPIRY_DAYS = 30;
export const TOKEN_NAME_MAX = 100;
