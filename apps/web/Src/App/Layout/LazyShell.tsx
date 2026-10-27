import { Suspense } from 'react';

import { PageLoading } from '@/App/Layout/PageStates';
import type { TFrameId } from '@/App/Panels';
import { lazyComponent } from '@/Core/Ui/LazyComponent';

const AppShell = lazyComponent(() => import('@/App/Layout/AppShell').then((module) => module.AppShell));

export const preloadShell = (stack: readonly TFrameId[]): void => {
  void AppShell.preload();
  if (stack.length === 0) return;
  void import('@/App/Layout/ShellParts').then((parts) => parts.preloadStack(stack));
};

export const LazyAppShell = () => (
  <Suspense fallback={<PageLoading />}>
    <AppShell />
  </Suspense>
);
