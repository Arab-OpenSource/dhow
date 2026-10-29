import type { TRole } from '@/Core/Auth/AuthTypes';

export type TGate = (role: TRole | undefined) => boolean;

export const anyone: TGate = () => true;
export const member: TGate = (role) => role !== undefined;
