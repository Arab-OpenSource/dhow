import { Suspense, useEffect, useState, type ReactNode } from 'react';

import { PageError, PageLoading } from '@/App/Layout/PageStates';
import type { TFrameId } from '@/App/Panels';
import { lazyComponent } from '@/Core/Ui/LazyComponent';
import { documentLocale } from '@/I18n/Locale';
import { useCatalogStore } from '@/I18n/useCatalogStore';
import { useI18n } from '@/I18n/useI18n';
import { loadWorkspaceMessages } from '@/I18n/WorkspaceCatalog';

const AppShell = lazyComponent(() => import('@/App/Layout/AppShell').then((module) => module.AppShell));

export const preloadShell = (stack: readonly TFrameId[]): void => {
  void AppShell.preload();
  void loadWorkspaceMessages(documentLocale()).catch(() => undefined);
  if (stack.length === 0) return;
  void import('@/App/Layout/ShellParts').then((parts) => parts.preloadStack(stack));
};

const WorkspaceMessages = ({ children }: { children: ReactNode }) => {
  const { Locale } = useI18n();
  const ready = useCatalogStore((state) => state.Workspace[Locale] !== undefined);
  const [Failed, setFailed] = useState(false);
  const [Attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (ready) return;
    loadWorkspaceMessages(Locale).catch(() => {
      setFailed(true);
    });
  }, [ready, Locale, Attempt]);

  if (ready) return children;
  if (Failed) {
    return (
      <PageError
        busy={false}
        onRetry={() => {
          setFailed(false);
          setAttempt((count) => count + 1);
        }}
      />
    );
  }
  return <PageLoading />;
};

export const LazyAppShell = () => (
  <WorkspaceMessages>
    <Suspense fallback={<PageLoading />}>
      <AppShell />
    </Suspense>
  </WorkspaceMessages>
);
