import { useNavigate } from '@tanstack/react-router';
import { Suspense } from 'react';

import { PageError, PageLoading } from '@/App/Layout/PageStates';
import { lazyComponent } from '@/Core/Ui/LazyComponent';
import { useMeta } from '@/Modules/System/Hooks/useMeta';

const SetupPage = lazyComponent(() => import('@/Modules/Identity/Pages/SetupPage').then((module) => module.SetupPage));

export const preloadSetupPage = (): void => {
  void SetupPage.preload();
};

export const SetupRoute = () => {
  const meta = useMeta();
  const navigate = useNavigate();

  if (meta.isError) {
    return (
      <PageError
        busy={meta.isFetching}
        onRetry={() => {
          void meta.refetch();
        }}
      />
    );
  }
  if (meta.data === undefined) return <PageLoading />;

  return (
    <Suspense fallback={<PageLoading />}>
      <SetupPage
        setupRequired={meta.data.SetupRequired}
        onDone={(signedIn) => {
          void navigate({ to: signedIn ? '/' : '/login' });
        }}
        onLogin={() => {
          void navigate({ to: '/login' });
        }}
      />
    </Suspense>
  );
};
