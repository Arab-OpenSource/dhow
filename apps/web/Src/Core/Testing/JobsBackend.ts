import { backendFailure, type TRouteHandler } from '@/Core/Testing/Backend';
import { envelope, ORG_ID, USER_ID } from '@/Core/Testing/Fetch';

export const JOB_ID = '0198a000-0000-7000-8000-0000000000d1';
export const RUNNER_ID = '0198a000-0000-7000-8000-0000000000e1';
export const TEST_IMAGE = 'docker.io/library/alpine:3.20@sha256:beefdbd8a1da6d2915566fde36db9db0b524eb737fc57cd1367effd16dc0d06d';

export type TJobFixture = Record<string, unknown> & { Id: string; Name: string; State: string };

const minutesAgo = (minutes: number): string => new Date(Date.now() - minutes * 60_000).toISOString();

export const jobFixture = (overrides: Partial<TJobFixture> = {}): TJobFixture => ({
  Id: JOB_ID,
  OrganizationId: ORG_ID,
  Name: 'build-web',
  State: 'queued',
  Labels: [],
  Isolation: 'container',
  Priority: 0,
  Image: TEST_IMAGE,
  SpecDigest: 'ab'.repeat(32),
  AttemptCount: 0,
  MaxInfraRetries: 2,
  QueuedAt: minutesAgo(12),
  QueueDeadline: new Date(Date.now() + 86_400_000).toISOString(),
  CreatedAt: minutesAgo(12),
  CreatedByUserId: USER_ID,
  ...overrides,
});

export const jobsFixture = (count: number): TJobFixture[] =>
  Array.from({ length: count }, (_, index) =>
    jobFixture({
      Id: `0198a000-0000-7000-8000-${String(index).padStart(12, '0')}`,
      Name: `job-${String(index + 1)}`,
      State: index % 2 === 0 ? 'succeeded' : 'failed',
      FailureReason: index % 2 === 0 ? undefined : 'STEP_FAILED',
      AttemptCount: 1,
      StartedAt: minutesAgo(10),
      FinishedAt: minutesAgo(9),
    }),
  );

export const specFixture = (overrides: Record<string, unknown> = {}) => ({
  Image: TEST_IMAGE,
  Steps: [
    { Name: 'install', Run: 'npm ci', Mutating: false },
    { Name: 'deploy', Run: 'kubectl apply -f k8s/', Mutating: true },
  ],
  Env: {},
  Secrets: [],
  Resources: { CpuMillis: 2000, MemoryBytes: 536_870_912, DiskBytes: 4_294_967_296, Pids: 1024 },
  TimeoutSeconds: 900,
  Network: { Egress: ['registry.npmjs.org', '*.github.com'] },
  Isolation: 'container',
  Labels: [],
  ...overrides,
});

export const attemptFixture = (overrides: Record<string, unknown> = {}) => ({
  Id: '0198a000-0000-7000-8000-0000000000f1',
  Number: 1,
  RunnerId: RUNNER_ID,
  FencingToken: 41,
  State: 'running',
  LeasedAt: minutesAgo(3),
  StartedAt: minutesAgo(3),
  Steps: [{ Index: 0, Name: 'install', Mutating: false, State: 'running', StartedAt: minutesAgo(3) }],
  ...overrides,
});

export interface IJobsBackend {
  jobs: TJobFixture[];
  attempts?: Record<string, unknown>[];
  spec?: Record<string, unknown>;
  detail?: (request: Request) => Response | Promise<Response> | undefined;
  list?: (request: Request, url: URL) => Response | Promise<Response> | undefined;
}

const pageOf = (items: readonly TJobFixture[], url: URL): Response => {
  const page = Number(url.searchParams.get('Page') ?? '1');
  const limit = Number(url.searchParams.get('Limit') ?? '25');
  const state = url.searchParams.get('State');
  const matching = state === null ? items : items.filter((item) => item.State === state);
  return envelope({ Success: true, Data: matching.slice((page - 1) * limit, page * limit), Meta: { Page: page, Limit: limit, Total: matching.length } });
};

const JOB_PATH = /\/organizations\/[0-9a-f-]{36}\/jobs\/([0-9a-f-]{36})(\/cancel)?$/;

export const jobsRoute = (backend: IJobsBackend): TRouteHandler => (request, url) => {
  if (url.pathname.endsWith('/jobs') && request.method === 'GET') return backend.list?.(request, url) ?? pageOf(backend.jobs, url);
  const match = JOB_PATH.exec(url.pathname);
  if (match === null) return undefined;
  const job = backend.jobs.find((item) => item.Id === match[1]);
  if (job === undefined) return backendFailure('NOT_FOUND', 'غير موجود.', 404);
  if (match[2] === '/cancel' && request.method === 'POST') {
    const now = new Date().toISOString();
    Object.assign(job, job.State === 'queued' ? { State: 'canceled', FailureReason: 'CANCELED', FinishedAt: now } : {});
    Object.assign(job, { CancelRequestedAt: now });
    return envelope({ Success: true, Data: job });
  }
  return (
    backend.detail?.(request) ??
    envelope({ Success: true, Data: { Job: job, Spec: specFixture(backend.spec), Attempts: backend.attempts ?? [] } })
  );
};
