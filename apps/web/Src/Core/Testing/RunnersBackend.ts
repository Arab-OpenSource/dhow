import { backendFailure, type TRouteHandler } from '@/Core/Testing/Backend';
import { envelope, ORG_ID } from '@/Core/Testing/Fetch';

export type TRunnerFixture = Record<string, unknown> & { Id: string; Name: string };

export const runnerId = (index: number): string => `0198a000-0000-7000-8000-0000000001${String(index).padStart(2, '0')}`;

export const runnerFixture = (overrides: Partial<TRunnerFixture> = {}): TRunnerFixture => ({
  Id: runnerId(1),
  OrganizationId: ORG_ID,
  Name: 'build-01',
  Labels: ['linux-builders'],
  CapabilityLabels: ['dhow.os=linux', 'dhow.arch=amd64', 'dhow.isolation=container'],
  Os: 'linux',
  Arch: 'amd64',
  Version: 'v0.3.0',
  IsolationLevels: ['container'],
  Capacity: 2,
  State: 'active',
  Status: 'online',
  KeyVersion: 1,
  KeysRotatedAt: '2026-10-28T08:00:00Z',
  LastSeenAt: new Date().toISOString(),
  CreatedAt: '2026-10-28T08:00:00Z',
  ...overrides,
});

export interface IRunnersBackend {
  runners: TRunnerFixture[];
  list?: (request: Request, url: URL) => Response | Promise<Response> | undefined;
}

const RUNNER_PATH = /\/organizations\/[0-9a-f-]{36}\/runners\/([0-9a-f-]{36})$/;

export const runnersRoute =
  (backend: IRunnersBackend): TRouteHandler =>
  (request, url) => {
    if (request.method !== 'GET') return undefined;
    if (url.pathname.endsWith('/runners')) {
      const custom = backend.list?.(request, url);
      if (custom !== undefined) return custom;
      const page = Number(url.searchParams.get('Page') ?? '1');
      const limit = Number(url.searchParams.get('Limit') ?? '25');
      return envelope({
        Success: true,
        Data: backend.runners.slice((page - 1) * limit, page * limit),
        Meta: { Page: page, Limit: limit, Total: backend.runners.length },
      });
    }
    const match = RUNNER_PATH.exec(url.pathname);
    if (match === null) return undefined;
    const runner = backend.runners.find((item) => item.Id === match[1]);
    return runner === undefined ? backendFailure('NOT_FOUND', 'غير موجود.', 404) : envelope({ Success: true, Data: runner });
  };
