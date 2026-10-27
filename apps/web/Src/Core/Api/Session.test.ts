import { afterEach, describe, expect, it } from 'vitest';

import { ApiError } from '@/Core/Api/ApiError';
import { ApiClient, request, requestPage, requestVoid } from '@/Core/Api/Client';
import { useCsrfStore } from '@/Core/Store/useCsrfStore';
import { envelope, meBody, stubFetch } from '@/Core/Testing/Fetch';

const ORG = '0198a000-0000-7000-8000-0000000000a1';

const logout = () => requestVoid(() => ApiClient.DELETE('/sessions/current'));

afterEach(() => {
  useCsrfStore.getState().Clear();
});

describe('the session in the API client', () => {
  it('sends cookies only to the same origin', async () => {
    const { calls } = stubFetch(() => envelope(meBody()));
    await request(() => ApiClient.GET('/me'));
    expect(calls[0]?.credentials).toBe('same-origin');
  });

  it('sends the CSRF token on every non-GET request and on no GET request', async () => {
    useCsrfStore.getState().Set('token-a');
    const { calls } = stubFetch(() => envelope({ Success: true, Data: { Revoked: 1 } }));
    await request(() => ApiClient.GET('/sessions'));
    await request(() => ApiClient.POST('/sessions/revoke-others'));
    await request(() => ApiClient.PUT('/me/password', { body: { CurrentPassword: 'a', NewPassword: 'b' } }));
    await request(() => ApiClient.DELETE('/sessions/{id}', { params: { path: { id: 'x' } } }));
    expect(calls.map((call) => [call.method, call.headers.get('X-CSRF-Token')])).toEqual([
      ['GET', null],
      ['POST', 'token-a'],
      ['PUT', 'token-a'],
      ['DELETE', 'token-a'],
    ]);
  });

  it('sends no token header while it holds none', async () => {
    const { calls } = stubFetch(() => envelope({ Success: true, Data: { Revoked: 0 } }));
    await request(() => ApiClient.POST('/sessions/revoke-others'));
    expect(calls[0]?.headers.has('X-CSRF-Token')).toBe(false);
  });

  it('never writes the token to browser storage or a cookie', async () => {
    useCsrfStore.getState().Set('token-secret');
    stubFetch(() => envelope({ Success: true, Data: { Revoked: 0 } }));
    await request(() => ApiClient.POST('/sessions/revoke-others'));
    expect(JSON.stringify(Object.entries(window.localStorage))).not.toContain('token-secret');
    expect(JSON.stringify(Object.entries(window.sessionStorage))).not.toContain('token-secret');
    expect(document.cookie).not.toContain('token-secret');
  });

  it('refetches /me once on CSRF_FAILED and retries with the fresh token', async () => {
    useCsrfStore.getState().Set('stale');
    const { calls } = stubFetch((incoming) => {
      const { pathname } = new URL(incoming.url);
      if (pathname.endsWith('/me')) return envelope(meBody('owner', 'fresh'));
      if (incoming.headers.get('X-CSRF-Token') !== 'fresh') {
        return envelope({ Success: false, Error: { Code: 'CSRF_FAILED', Message: 'x' } }, 403);
      }
      return envelope({ Success: true, Data: { Revoked: 2 } });
    });
    await expect(request(() => ApiClient.POST('/sessions/revoke-others'))).resolves.toEqual({ Revoked: 2 });
    expect(calls.map((call) => `${call.method} ${new URL(call.url).pathname}`)).toEqual([
      'POST /api/v1/sessions/revoke-others',
      'GET /api/v1/me',
      'POST /api/v1/sessions/revoke-others',
    ]);
    expect(useCsrfStore.getState().Token).toBe('fresh');
  });

  it('fails visibly when the retry is refused too, after only one retry', async () => {
    useCsrfStore.getState().Set('stale');
    const { calls } = stubFetch((incoming) =>
      new URL(incoming.url).pathname.endsWith('/me')
        ? envelope(meBody('owner', 'still-wrong'))
        : envelope({ Success: false, Error: { Code: 'CSRF_FAILED', Message: 'x' } }, 403),
    );
    const failure = await request(() => ApiClient.POST('/sessions/revoke-others')).catch((error: unknown) => error);
    expect(failure).toBeInstanceOf(ApiError);
    expect(failure).toMatchObject({ Code: 'CSRF_FAILED', Status: 403 });
    expect(calls.filter((call) => call.method === 'POST')).toHaveLength(2);
  });

  it('does not retry a CSRF failure when it holds no token', async () => {
    const { calls } = stubFetch(() => envelope({ Success: false, Error: { Code: 'CSRF_FAILED', Message: 'x' } }, 403));
    await expect(request(() => ApiClient.POST('/sessions/revoke-others'))).rejects.toMatchObject({ Code: 'CSRF_FAILED' });
    expect(calls).toHaveLength(1);
  });

});

describe('what the API client reads from a reply', () => {
  it('reads Retry-After and the field details from a failed reply', async () => {
    stubFetch(
      () =>
        new Response(
          JSON.stringify({
            Success: false,
            Error: { Code: 'INVALID_CREDENTIALS', Message: 'x', Details: [{ Field: 'Email', Message: 'bad' }] },
          }),
          { status: 401, headers: { 'Content-Type': 'application/json', 'Retry-After': '30' } },
        ),
    );
    const failure = await request<unknown>(() => ApiClient.POST('/sessions', { body: { Email: 'a@b.c', Password: 'x' } })).catch(
      (error: unknown) => error,
    );
    expect(failure).toMatchObject({
      Code: 'INVALID_CREDENTIALS',
      Status: 401,
      RetryAfterSeconds: 30,
      Details: [{ Field: 'Email', Message: 'bad' }],
    });
  });

  it('leaves RetryAfterSeconds empty without the header', async () => {
    stubFetch(() => envelope({ Success: false, Error: { Code: 'INVALID_CREDENTIALS', Message: 'x' } }, 401));
    await expect(request<unknown>(() => ApiClient.POST('/sessions', { body: { Email: 'a@b.c', Password: 'x' } }))).rejects.toMatchObject({
      RetryAfterSeconds: null,
    });
  });

  it('returns a page with its Meta', async () => {
    stubFetch(() => envelope({ Success: true, Data: [], Meta: { Page: 2, Limit: 20, Total: 41 } }));
    const page = await requestPage(() => ApiClient.GET('/organizations/{id}/members', { params: { path: { id: ORG } } }));
    expect(page).toEqual({ Items: [], Meta: { Page: 2, Limit: 20, Total: 41 } });
  });

  it('accepts a reply with no Data when the caller expects none', async () => {
    stubFetch(() => envelope({ Success: true, Message: 'session.logout.succeeded' }));
    await expect(logout()).resolves.toBeUndefined();
  });

  it('turns a refused no-Data call into an ApiError', async () => {
    stubFetch(() => envelope({ Success: false, Error: { Code: 'UNAUTHENTICATED', Message: 'x' } }, 401));
    await expect(logout()).rejects.toMatchObject({ Code: 'UNAUTHENTICATED', Status: 401 });
  });
});
