import { createRootRoute, createRoute, createRouter, type RouterHistory } from '@tanstack/react-router';

import { AppShell } from '@/App/Layout/AppShell';
import { NotFoundPage } from '@/App/Layout/NotFoundPage';
import { parseShellSearch } from '@/App/Panels';
import { WorkspacePage } from '@/Modules/System/Pages/WorkspacePage';

const rootRoute = createRootRoute({
  component: AppShell,
  notFoundComponent: NotFoundPage,
  validateSearch: parseShellSearch,
});

const workspaceRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: WorkspacePage,
});

const routeTree = rootRoute.addChildren([workspaceRoute]);

export const createAppRouter = (history?: RouterHistory) => createRouter({ routeTree, history });
