import { useLocation, useNavigate } from '@tanstack/react-router';
import { useEffect, useState } from 'react';

import { PageError, PageLoading } from '@/App/Layout/PageStates';
import { useCsrfStore } from '@/Core/Store/useCsrfStore';
import { useMeta } from '@/Modules/System/Hooks/useMeta';

export const EntryRedirect = () => {
  const meta = useMeta();
  const navigate = useNavigate();
  const { href } = useLocation();
  const [From] = useState(href);
  const setupRequired = meta.data?.SetupRequired;

  useEffect(() => {
    useCsrfStore.getState().Clear();
  }, []);

  useEffect(() => {
    if (setupRequired === undefined) return;
    if (setupRequired) void navigate({ to: '/setup', replace: true });
    else void navigate({ to: '/login', search: From === '/' ? {} : { redirect: From }, replace: true });
  }, [setupRequired, From, navigate]);

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
  return <PageLoading />;
};
