import type { TRouteHandler } from '@/Core/Testing/Backend';
import { envelope } from '@/Core/Testing/Fetch';

type TSource = 'env' | 'database' | 'default';

const value = (Value: string | number | boolean, Env: string, Source: TSource = 'default', LockedBy?: string) => ({
  Value,
  Source,
  Locked: Source === 'env' || LockedBy !== undefined,
  Env,
  ...(Source === 'env' ? { LockedBy: Env } : LockedBy === undefined ? {} : { LockedBy }),
});

export const settingsFixture = () => ({
  Mail: {
    Version: 0,
    Host: value('', 'DHOW_SMTP_HOST'),
    Port: value(587, 'DHOW_SMTP_PORT'),
    Tls: value('starttls', 'DHOW_SMTP_TLS'),
    Username: value('', 'DHOW_SMTP_USERNAME'),
    Password: { Set: false, Locked: false, Saved: false, Env: 'DHOW_SMTP_PASSWORD' },
    From: value('', 'DHOW_SMTP_FROM'),
  },
  Organizations: { Version: 0, Creation: value('instance-owner', 'DHOW_ORG_CREATION') },
  Passwords: { Version: 0, BreachedCheck: value(true, 'DHOW_BREACHED_PASSWORD_CHECK') },
  RateLimits: {
    Version: 0,
    Public: value(300, 'DHOW_RATE_LIMIT_PUBLIC'),
    InvitesPerUser: value(50, 'DHOW_RATE_LIMIT_INVITES_USER'),
    InvitesPerOrganization: value(200, 'DHOW_RATE_LIMIT_INVITES_ORG'),
  },
  Logs: { Version: 0, RetentionDays: value(30, 'DHOW_LOG_RETENTION_DAYS'), JobLogMaxBytes: value(67108864, 'DHOW_JOB_LOG_MAX_BYTES') },
});

export type TSettingsFixture = ReturnType<typeof settingsFixture>;

export const configuredSettings = (): TSettingsFixture => {
  const s = settingsFixture();
  s.Mail = {
    ...s.Mail,
    Version: 2,
    Host: value('smtp.example.com', 'DHOW_SMTP_HOST', 'database'),
    Username: value('dhow', 'DHOW_SMTP_USERNAME', 'database'),
    Password: { Set: true, Locked: false, Saved: true, Env: 'DHOW_SMTP_PASSWORD' },
    From: value('Dhow <dhow@example.com>', 'DHOW_SMTP_FROM', 'database'),
  };
  s.RateLimits = { ...s.RateLimits, Public: value(1000, 'DHOW_RATE_LIMIT_PUBLIC', 'env') };
  return s;
};

export const passwordFromEnv = (): TSettingsFixture => {
  const s = configuredSettings();
  s.Mail = {
    ...s.Mail,
    Host: value('smtp.example.com', 'DHOW_SMTP_HOST', 'database', 'DHOW_SMTP_PASSWORD'),
    Port: value(587, 'DHOW_SMTP_PORT', 'default', 'DHOW_SMTP_PASSWORD'),
    Tls: value('starttls', 'DHOW_SMTP_TLS', 'default', 'DHOW_SMTP_PASSWORD'),
    Password: { Set: true, Locked: true, Saved: true, Env: 'DHOW_SMTP_PASSWORD' },
  };
  return s;
};

export interface ISettingsBackend {
  settings: TSettingsFixture;
  puts: { path: string; body: Record<string, unknown> }[];
  tests: number;
}

export interface ISettingsReplies {
  get?: () => Response;
  put?: (path: string, body: Record<string, unknown>) => Response | undefined;
  test?: () => Response;
}

const saved = async (state: ISettingsBackend, replies: ISettingsReplies, request: Request, path: string): Promise<Response> => {
  const body = (await request.clone().json()) as Record<string, unknown>;
  state.puts.push({ path, body });
  const custom = replies.put?.(path, body);
  if (custom) return custom;
  state.settings.Mail.Version += 1;
  return envelope({ Success: true, Message: 'settings.saved', Data: state.settings });
};

export const settingsRoute =
  (state: ISettingsBackend, replies: ISettingsReplies = {}): TRouteHandler =>
  (request, url) => {
    if (!url.pathname.startsWith('/api/v1/instance/settings')) return undefined;
    if (request.method === 'GET') return replies.get?.() ?? envelope({ Success: true, Data: state.settings });
    if (url.pathname.endsWith('/mail/test')) {
      state.tests += 1;
      return replies.test?.() ?? envelope({ Success: true, Data: { SentTo: 'sabry@future.dev' } });
    }
    return saved(state, replies, request, url.pathname);
  };
