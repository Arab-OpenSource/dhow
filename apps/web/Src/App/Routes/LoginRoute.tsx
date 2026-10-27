import { Navigate, useNavigate, useRouter, useSearch } from '@tanstack/react-router';
import { lazy, Suspense } from 'react';

import { PageError, PageLoading } from '@/App/Layout/PageStates';
import { safeRedirect } from '@/App/Routes/Redirect';
import { useMeta } from '@/Modules/System/Hooks/useMeta';

const LoginPage = lazy(() =>
  import('@/Modules/Session/Pages/LoginPage').then((module) => ({ default: module.LoginPage })),
);

export const LoginRoute = () => {
  const meta = useMeta();
  const navigate = useNavigate();
  const router = useRouter();
  const { redirect } = useSearch({ from: '/login' });

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
  if (meta.data.SetupRequired) return <Navigate to="/setup" replace />;

  return (
    <Suspense fallback={<PageLoading />}>
      <LoginPage
        onSignedIn={() => {
          const target = safeRedirect(redirect);
          if (target === null) void navigate({ to: '/' });
          else router.history.push(target);
        }}
      />
    </Suspense>
  );
};
