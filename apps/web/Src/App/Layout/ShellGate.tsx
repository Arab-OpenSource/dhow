import { EntryRedirect } from '@/App/Layout/EntryRedirect';
import { LazyAppShell } from '@/App/Layout/LazyShell';
import { PageError, PageLoading } from '@/App/Layout/PageStates';
import { isUnauthenticated } from '@/Core/Api/ApiError';
import { useMe } from '@/Core/Auth/useMe';

export const ShellGate = () => {
  const me = useMe();

  if (isUnauthenticated(me.error)) return <EntryRedirect />;
  if (me.isError) {
    return (
      <PageError
        busy={me.isFetching}
        onRetry={() => {
          void me.refetch();
        }}
      />
    );
  }
  if (me.data === undefined) return <PageLoading />;
  return <LazyAppShell />;
};
