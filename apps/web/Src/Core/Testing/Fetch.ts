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

export const apiUp = (meta: Record<string, unknown> = META_BODY, ready: Record<string, unknown> = READY_BODY) =>
  stubFetch((request) => {
    const { pathname } = new URL(request.url);
    if (pathname.endsWith('/meta')) return envelope(meta);
    if (pathname.endsWith('/health/ready')) return envelope(ready);
    return envelope({ Success: false, Error: { Code: 'NOT_FOUND', Message: 'x' } }, 404);
  });

export const apiDown = () =>
  stubFetch(() => {
    throw new TypeError('Failed to fetch');
  });
