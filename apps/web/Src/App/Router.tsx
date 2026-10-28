import {
  createRootRoute,
  createRoute,
  createRouter,
  lazyRouteComponent,
  redirect,
  type RouterHistory,
} from '@tanstack/react-router';

import { preloadShell } from '@/App/Layout/LazyShell';
import { NotFoundPage } from '@/App/Layout/NotFoundPage';
import { RootLayout } from '@/App/Layout/RootLayout';
import { ShellGate } from '@/App/Layout/ShellGate';
import { parseShellSearch, stringifyShellSearch, UNKNOWN_STACK } from '@/App/Panels';
import { LoginRoute, preloadLoginPage } from '@/App/Routes/LoginRoute';
import { InviteRoute, preloadInvitePage } from '@/App/Routes/InviteRoute';
import { OidcCallbackRoute, preloadOidcCallbackPage } from '@/App/Routes/OidcCallbackRoute';
import { preloadSetupPage, SetupRoute } from '@/App/Routes/SetupRoute';
import { INVITE_PATH, takeInviteTokenFromHash } from '@/Modules/Identity/Store/InviteToken';
import { SETUP_PATH, takeSetupTokenFromHash } from '@/Modules/Identity/Store/SetupToken';
import { OIDC_CALLBACK_PATH, takeOidcCallbackFromSearch } from '@/Modules/Session/Store/OidcCallback';

const rootRoute = createRootRoute({
  component: RootLayout,
  notFoundComponent: NotFoundPage,
});

const shellRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'shell',
  component: ShellGate,
  validateSearch: parseShellSearch,
  loaderDeps: ({ search }) => ({ stack: search.stack }),
  loader: ({ deps }) => {
    preloadShell(deps.stack === undefined || deps.stack === UNKNOWN_STACK ? [] : deps.stack);
  },
});

const workspaceRoute = createRoute({
  getParentRoute: () => shellRoute,
  path: '/',
  component: lazyRouteComponent(() => import('@/Modules/System/Pages/WorkspacePage'), 'WorkspacePage'),
});

const shellNotFoundRoute = createRoute({
  getParentRoute: () => shellRoute,
  path: '$',
  component: NotFoundPage,
});

const setupRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: SETUP_PATH,
  beforeLoad: ({ location }) => {
    if (takeSetupTokenFromHash(location.hash)) throw redirect({ to: SETUP_PATH, hash: '', replace: true });
  },
  loader: preloadSetupPage,
  component: SetupRoute,
});

const inviteRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: INVITE_PATH,
  beforeLoad: ({ location }) => {
    if (takeInviteTokenFromHash(location.hash)) throw redirect({ to: INVITE_PATH, hash: '', replace: true });
  },
  loader: preloadInvitePage,
  component: InviteRoute,
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  validateSearch: (search: Record<string, unknown>): { redirect?: string } =>
    typeof search.redirect === 'string' ? { redirect: search.redirect } : {},
  loader: preloadLoginPage,
  component: LoginRoute,
});

const oidcCallbackRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: OIDC_CALLBACK_PATH,
  beforeLoad: ({ location, params }) => {
    if (takeOidcCallbackFromSearch(location.pathname, location.searchStr)) {
      throw redirect({ to: OIDC_CALLBACK_PATH, params: { provider: params.provider }, search: {}, replace: true });
    }
  },
  loader: preloadOidcCallbackPage,
  component: OidcCallbackRoute,
});

const routeTree = rootRoute.addChildren([
  shellRoute.addChildren([workspaceRoute, shellNotFoundRoute]),
  setupRoute,
  inviteRoute,
  loginRoute,
  oidcCallbackRoute,
]);

export const createAppRouter = (history?: RouterHistory) =>
  createRouter({ routeTree, history, stringifySearch: stringifyShellSearch });
