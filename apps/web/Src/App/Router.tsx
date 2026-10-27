import { createRootRoute, createRoute, createRouter, redirect, type RouterHistory } from '@tanstack/react-router';

import { NotFoundPage } from '@/App/Layout/NotFoundPage';
import { RootLayout } from '@/App/Layout/RootLayout';
import { ShellGate } from '@/App/Layout/ShellGate';
import { parseShellSearch, stringifyShellSearch } from '@/App/Panels';
import { LoginRoute } from '@/App/Routes/LoginRoute';
import { SetupRoute } from '@/App/Routes/SetupRoute';
import { SETUP_PATH, takeSetupTokenFromHash } from '@/Modules/Identity/Store/SetupToken';
import { WorkspacePage } from '@/Modules/System/Pages/WorkspacePage';

const rootRoute = createRootRoute({
  component: RootLayout,
  notFoundComponent: NotFoundPage,
});

const shellRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'shell',
  component: ShellGate,
  validateSearch: parseShellSearch,
});

const workspaceRoute = createRoute({
  getParentRoute: () => shellRoute,
  path: '/',
  component: WorkspacePage,
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
  component: SetupRoute,
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  validateSearch: (search: Record<string, unknown>): { redirect?: string } =>
    typeof search.redirect === 'string' ? { redirect: search.redirect } : {},
  component: LoginRoute,
});

const routeTree = rootRoute.addChildren([shellRoute.addChildren([workspaceRoute, shellNotFoundRoute]), setupRoute, loginRoute]);

export const createAppRouter = (history?: RouterHistory) =>
  createRouter({ routeTree, history, stringifySearch: stringifyShellSearch });
