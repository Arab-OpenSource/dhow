import type { TRole } from '@/Core/Auth/AuthTypes';

export type TGate = (role: TRole | undefined, instanceOwner?: boolean) => boolean;

export const anyone: TGate = () => true;
export const member: TGate = (role) => role !== undefined;
export const instanceOwner: TGate = (_role, owner = false) => owner;
