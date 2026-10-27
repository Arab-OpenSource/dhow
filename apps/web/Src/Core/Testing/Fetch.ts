import { vi } from 'vitest';

export const envelope = (body: Record<string, unknown>, status = 200): Response =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

export type TFetchHandler = (request: Request) => Response | Promise<Response>;

export const stubFetch = (handler: TFetchHandler) => {
  const calls: Request[] = [];
  const stub = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const request = input instanceof Request ? input : new Request(input, init);
    calls.push(request);
    return Promise.resolve(handler(request));
  });
  vi.stubGlobal('fetch', stub);
  return { stub, calls };
};

export const META_BODY = { Success: true, Message: 'system.meta.loaded', Data: { Version: 'v0.1.0', Commit: '2e39188', SetupRequired: false } };

export const READY_BODY = {
  Success: true,
  Data: {
    Status: 'ok',
    Checks: [
      { Name: 'database', Ok: true },
      { Name: 'migrations', Ok: true },
    ],
  },
};

export const ORG_ID = '0198a000-0000-7000-8000-0000000000a1';
export const USER_ID = '0198a000-0000-7000-8000-0000000000b1';
export const SESSION_ID = '0198a000-0000-7000-8000-0000000000c1';
export const CSRF_TOKEN = 'csrf-token-1';

export const meBody = (role = 'owner', csrfToken = CSRF_TOKEN) => ({
  Success: true,
  Message: 'session.me.loaded',
  Data: {
    User: { Id: USER_ID, Email: 'sabry@future.dev', Name: 'Sabry Dawood', Locale: 'ar' },
    Memberships: [{ OrganizationId: ORG_ID, OrganizationName: 'Future Solution', OrganizationSlug: 'future-solution', Role: role }],
    CsrfToken: csrfToken,
    SessionId: SESSION_ID,
    ExpiresAt: '2026-11-10T12:00:00Z',
  },
});

export const UNAUTHENTICATED_BODY = {
  Success: false,
  Error: { Code: 'UNAUTHENTICATED', Message: 'سجّل الدخول الأول.' },
};

export const notFound = (): Response => envelope({ Success: false, Error: { Code: 'NOT_FOUND', Message: 'x' } }, 404);

export const apiUp = (meta: Record<string, unknown> = META_BODY, ready: Record<string, unknown> = READY_BODY, me: Record<string, unknown> | null = meBody()) =>
  stubFetch((request) => {
    const { pathname } = new URL(request.url);
    if (pathname.endsWith('/meta')) return envelope(meta);
    if (pathname.endsWith('/health/ready')) return envelope(ready);
    if (pathname.endsWith('/me')) return me === null ? envelope(UNAUTHENTICATED_BODY, 401) : envelope(me);
    return notFound();
  });

export const apiDown = () =>
  stubFetch(() => {
    throw new TypeError('Failed to fetch');
  });
