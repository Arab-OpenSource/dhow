import { QueryClientProvider, type QueryClient } from '@tanstack/react-query';
import { render, type RenderResult } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ReactElement } from 'react';

import { createQueryClient } from '@/Core/Api/QueryClient';
import { TooltipProvider } from '@/Core/Ui/Tooltip';
import { I18nProvider } from '@/I18n/I18nProvider';

export interface IUiRender extends RenderResult {
  user: ReturnType<typeof userEvent.setup>;
  client: QueryClient;
}

export interface IRenderOptions {
  advanceTimers?: (ms: number) => void | Promise<void>;
  client?: QueryClient;
}

export const renderUi = (ui: ReactElement): Omit<IUiRender, 'client'> => {
  const user = userEvent.setup();
  return { user, ...render(<TooltipProvider>{ui}</TooltipProvider>) };
};

export const renderWithProviders = (ui: ReactElement, options: IRenderOptions = {}): IUiRender => {
  const user = userEvent.setup(options.advanceTimers ? { advanceTimers: options.advanceTimers } : undefined);
  const client = options.client ?? createQueryClient({ retry: false });
  return {
    user,
    client,
    ...render(
      <I18nProvider>
        <QueryClientProvider client={client}>
          <TooltipProvider>{ui}</TooltipProvider>
        </QueryClientProvider>
      </I18nProvider>,
    ),
  };
};
