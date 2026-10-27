import type { components } from '@/Core/Api/Schema';

export type TMeta = components['schemas']['Meta'];
export type THealthStatus = components['schemas']['HealthStatus'];
export type THealthCheck = components['schemas']['HealthCheck'];
export type TCheckName = THealthCheck['Name'];

export const CHECK_NAMES = ['database', 'migrations'] as const satisfies readonly TCheckName[];

export const isCheckName = (value: unknown): value is TCheckName => CHECK_NAMES.some((name) => name === value);
