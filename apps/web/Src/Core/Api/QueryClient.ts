import { QueryClient } from '@tanstack/react-query';

export interface IQueryClientOptions {
  retry?: number | false;
}

export const createQueryClient = ({ retry = 1 }: IQueryClientOptions = {}): QueryClient =>
  new QueryClient({
    defaultOptions: {
      queries: { retry, refetchOnWindowFocus: false, staleTime: 10_000 },
    },
  });
