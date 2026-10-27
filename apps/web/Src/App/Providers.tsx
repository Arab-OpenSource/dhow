import { QueryClientProvider, type QueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';

import { I18nProvider } from '@/I18n/I18nProvider';

export interface IProvidersProps {
  queryClient: QueryClient;
  children: ReactNode;
}

export const Providers = ({ queryClient, children }: IProvidersProps) => (
  <I18nProvider>
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  </I18nProvider>
);
