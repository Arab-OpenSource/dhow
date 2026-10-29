import { ApiClient, request } from '@/Core/Api/Client';
import type { components } from '@/Core/Api/Schema';

type TSchemas = components['schemas'];

export type TOverview = TSchemas['OrganizationOverview'];
export type TOverviewJobs = NonNullable<TSchemas['OverviewJobs']>;
export type TOverviewRunners = NonNullable<TSchemas['OverviewRunners']>;
export type TOverviewInvitations = NonNullable<TSchemas['OverviewInvitations']>;

export const getOverview = (organizationId: string): Promise<TOverview> =>
  request(() => ApiClient.GET('/organizations/{id}/overview', { params: { path: { id: organizationId } } }));
