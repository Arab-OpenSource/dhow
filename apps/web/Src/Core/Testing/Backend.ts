import { envelope, META_BODY, meBody, ORG_ID, READY_BODY, SESSION_ID, stubFetch, UNAUTHENTICATED_BODY } from '@/Core/Testing/Fetch';

export interface ISessionFixture {
  Id: string;
  CreatedAt: string;
  LastSeenAt: string;
  ExpiresAt: string;
  UserAgent: string;
  IpAddress: string;
  Current: boolean;
}

export const CHROME_UA = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36';
export const FIREFOX_UA = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0';
export const SAFARI_UA =
  'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1';

export const sessionsFixture = (): ISessionFixture[] => [
  {
    Id: SESSION_ID,
    CreatedAt: '2026-10-27T08:00:00Z',
    LastSeenAt: new Date().toISOString(),
    ExpiresAt: '2026-11-10T08:00:00Z',
    UserAgent: CHROME_UA,
    IpAddress: '203.0.113.7',
    Current: true,
  },
  {
    Id: '0198a000-0000-7000-8000-0000000000c2',
    CreatedAt: '2026-10-26T08:00:00Z',
    LastSeenAt: new Date(Date.now() - 2 * 3_600_000).toISOString(),
    ExpiresAt: '2026-11-09T08:00:00Z',
    UserAgent: FIREFOX_UA,
    IpAddress: '198.51.100.20',
    Current: false,
  },
  {
    Id: '0198a000-0000-7000-8000-0000000000c3',
    CreatedAt: '2026-10-25T08:00:00Z',
    LastSeenAt: new Date(Date.now() - 26 * 3_600_000).toISOString(),
    ExpiresAt: '2026-11-08T08:00:00Z',
    UserAgent: SAFARI_UA,
    IpAddress: '2001:db8::1',
    Current: false,
  },
];

export type TRouteHandler = (request: Request, url: URL) => Response | undefined;

export interface IBackendOptions {
  role?: string;
  signedIn?: boolean;
  setupRequired?: boolean;
  sessions?: ISessionFixture[];
  routes?: TRouteHandler[];
  passwordReply?: () => Response;
}

export interface IBackendState {
  signedIn: boolean;
  sessions: ISessionFixture[];
  role: string;
  csrf: string;
}

const failure = (Code: string, Message: string, status: number, headers: Record<string, string> = {}): Response =>
  new Response(JSON.stringify({ Success: false, Error: { Code, Message } }), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  });

export const backendFailure = failure;

const SESSION_PATH = /\/sessions\/([0-9a-f-]{36})$/;

export const startBackend = (options: IBackendOptions = {}) => {
  const state: IBackendState = {
    signedIn: options.signedIn ?? true,
    sessions: options.sessions ?? sessionsFixture(),
    role: options.role ?? 'owner',
    csrf: 'csrf-token-1',
  };
  const stub = stubFetch((request) => {
    const url = new URL(request.url);
    const { pathname } = url;
    for (const route of options.routes ?? []) {
      const handled = route(request, url);
      if (handled) return handled;
    }
    if (pathname.endsWith('/meta')) {
      return envelope({ ...META_BODY, Data: { ...META_BODY.Data, SetupRequired: options.setupRequired ?? false } });
    }
    if (pathname.endsWith('/health/ready')) return envelope(READY_BODY);
    if (pathname.endsWith('/me') && request.method === 'GET') {
      return state.signedIn ? envelope(meBody(state.role, state.csrf)) : envelope(UNAUTHENTICATED_BODY, 401);
    }
    if (pathname.endsWith('/me/password') && request.method === 'PUT') {
      if (options.passwordReply) return options.passwordReply();
      const revoked = state.sessions.filter((session) => !session.Current).length;
      state.sessions = state.sessions.filter((session) => session.Current);
      return envelope({ Success: true, Data: { Revoked: revoked } });
    }
    if (pathname.endsWith('/sessions') && request.method === 'GET') return envelope({ Success: true, Data: state.sessions });
    if (pathname.endsWith('/sessions/current') && request.method === 'DELETE') {
      state.signedIn = false;
      return envelope({ Success: true, Message: 'session.logout.succeeded' });
    }
    if (pathname.endsWith('/sessions/revoke-others') && request.method === 'POST') {
      const revoked = state.sessions.filter((session) => !session.Current).length;
      state.sessions = state.sessions.filter((session) => session.Current);
      return envelope({ Success: true, Data: { Revoked: revoked } });
    }
    const target = SESSION_PATH.exec(pathname)?.[1];
    if (target !== undefined && request.method === 'DELETE') {
      state.sessions = state.sessions.filter((session) => session.Id !== target);
      return envelope({ Success: true, Message: 'session.revoke.succeeded' });
    }
    return failure('NOT_FOUND', 'x', 404);
  });
  return { ...stub, state, orgId: ORG_ID };
};
