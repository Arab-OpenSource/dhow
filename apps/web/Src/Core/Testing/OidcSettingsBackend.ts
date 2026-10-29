import type { TRouteHandler } from "@/Core/Testing/Backend";
import { envelope } from "@/Core/Testing/Fetch";

const BASE = "https://dhow.example.com/oidc/callback/";

export const oidcProviderFixture = (
  id = "corp",
  overrides: Record<string, unknown> = {},
) => ({
  Id: id,
  Type: "oidc",
  Name: "Corp SSO",
  Issuer: "https://sso.corp.example",
  ClientId: "dhow-client",
  ClientSecret: {
    Set: true,
    Saved: true,
    Locked: false,
    Env: `DHOW_OIDC_${id.toUpperCase()}_CLIENT_SECRET`,
  },
  AllowedDomains: ["corp.example"],
  Enabled: true,
  Version: 1,
  LinkedIdentities: 0,
  OnlySignInUsers: 0,
  CallbackUrl: `${BASE}${id}`,
  ...overrides,
});

export type TOidcProviderFixture = ReturnType<typeof oidcProviderFixture>;

export const oidcFixture = (
  providers: TOidcProviderFixture[] = [],
  locked = false,
) => ({
  Source: locked ? "env" : "database",
  Locked: locked,
  Env: "DHOW_OIDC_PROVIDERS",
  CallbackBase: BASE,
  MaxProviders: 10,
  IgnoredStored: 0,
  Providers: providers,
});

export type TOidcFixture = ReturnType<typeof oidcFixture>;

export interface IOidcBackend {
  oidc: TOidcFixture;
  writes: { method: string; path: string; body: Record<string, unknown> }[];
  tests: { Type: string; Issuer: string }[];
}

export interface IOidcReplies {
  get?: () => Response;
  write?: (
    method: string,
    path: string,
    body: Record<string, unknown>,
  ) => Response | undefined;
  test?: () => Response;
}

const PATH = "/api/v1/instance/settings/oidc";

const written = (
  state: IOidcBackend,
  method: string,
  path: string,
  body: Record<string, unknown>,
): TOidcFixture => {
  const id = path.slice(PATH.length + 1).split("/")[0] ?? "";
  if (method === "POST" && path === PATH) {
    const created = oidcProviderFixture(String(body.Id), {
      Type: body.Type,
      Name: body.Name,
      Issuer: body.Issuer,
      ClientId: body.ClientId,
    });
    return { ...state.oidc, Providers: [...state.oidc.Providers, created] };
  }
  if (path.endsWith("/remove"))
    return {
      ...state.oidc,
      Providers: state.oidc.Providers.filter((p) => p.Id !== id),
    };
  const providers = state.oidc.Providers.map((p) =>
    p.Id === id
      ? {
          ...p,
          Name: typeof body.Name === "string" ? body.Name : p.Name,
          Version: p.Version + 1,
        }
      : p,
  );
  return { ...state.oidc, Providers: providers };
};

const PASSED = {
  Passed: true,
  Checks: [
    { Check: "discovery", Passed: true },
    { Check: "endpoints", Passed: true },
    { Check: "keys", Passed: true },
  ],
};

const handled = async (
  state: IOidcBackend,
  replies: IOidcReplies,
  request: Request,
  url: URL,
): Promise<Response> => {
  const body = (await request.clone().json()) as Record<string, unknown>;
  if (url.pathname === `${PATH}/test`) {
    state.tests.push({ Type: String(body.Type), Issuer: String(body.Issuer) });
    return replies.test?.() ?? envelope({ Success: true, Data: PASSED });
  }
  state.writes.push({ method: request.method, path: url.pathname, body });
  const custom = replies.write?.(request.method, url.pathname, body);
  if (custom) return custom;
  state.oidc = written(state, request.method, url.pathname, body);
  return envelope({
    Success: true,
    Message: "settings.oidc.saved",
    Data: state.oidc,
  });
};

export const oidcRoute =
  (state: IOidcBackend, replies: IOidcReplies = {}): TRouteHandler =>
  (request, url) => {
    if (!url.pathname.startsWith(PATH)) return undefined;
    if (request.method === "GET")
      return replies.get?.() ?? envelope({ Success: true, Data: state.oidc });
    return handled(state, replies, request, url);
  };
