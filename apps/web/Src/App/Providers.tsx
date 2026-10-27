import { QueryClientProvider, type QueryClient } from '@tanstack/react-query';
import { MotionConfig } from 'motion/react';
import type { ReactNode } from 'react';

import { TooltipProvider } from '@/Core/Ui/Tooltip';
import { I18nProvider } from '@/I18n/I18nProvider';

export interface IProvidersProps {
  queryClient: QueryClient;
  children: ReactNode;
}

export const Providers = ({ queryClient, children }: IProvidersProps) => (
  <I18nProvider>
    <MotionConfig reducedMotion="user">
      <QueryClientProvider client={queryClient}>
        <TooltipProvider>{children}</TooltipProvider>
      </QueryClientProvider>
    </MotionConfig>
  </I18nProvider>
);
