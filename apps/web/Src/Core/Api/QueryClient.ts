import { MutationCache, QueryCache, QueryClient, type QueryKey } from '@tanstack/react-query';

import { isApiError, isUnauthenticated } from '@/Core/Api/ApiError';
import { ME_QUERY_KEY } from '@/Core/Auth/MeQuery';

export interface IQueryClientOptions {
  retry?: number | false;
}

const DEFAULT_RETRIES = 1;
const DEFAULT_STALE_MS = 30_000;

const retryServerErrors = (failureCount: number, error: unknown): boolean =>
  failureCount < DEFAULT_RETRIES && !(isApiError(error) && error.Status >= 400 && error.Status < 500);

const isMeKey = (key: QueryKey): boolean => key.length === ME_QUERY_KEY.length && key.every((part, i) => part === ME_QUERY_KEY[i]);

export const createQueryClient = ({ retry }: IQueryClientOptions = {}): QueryClient => {
  const client: QueryClient = new QueryClient({
    queryCache: new QueryCache({
      onError: (error, query) => {
        if (isUnauthenticated(error) && !isMeKey(query.queryKey)) {
          void client.invalidateQueries({ queryKey: ME_QUERY_KEY });
        }
      },
    }),
    mutationCache: new MutationCache({
      onError: (error) => {
        if (isUnauthenticated(error)) void client.invalidateQueries({ queryKey: ME_QUERY_KEY });
      },
    }),
    defaultOptions: {
      queries: { retry: retry ?? retryServerErrors, refetchOnWindowFocus: false, staleTime: DEFAULT_STALE_MS },
    },
  });
  return client;
};
