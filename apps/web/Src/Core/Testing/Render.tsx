import { QueryClientProvider } from '@tanstack/react-query';
import { render, type RenderResult } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ReactElement } from 'react';

import { createQueryClient } from '@/Core/Api/QueryClient';
import { TooltipProvider } from '@/Core/Ui/Tooltip';
import { I18nProvider } from '@/I18n/I18nProvider';

export interface IUiRender extends RenderResult {
  user: ReturnType<typeof userEvent.setup>;
}

export const renderUi = (ui: ReactElement): IUiRender => {
  const user = userEvent.setup();
  return { user, ...render(<TooltipProvider>{ui}</TooltipProvider>) };
};

export const renderWithProviders = (ui: ReactElement): IUiRender => {
  const user = userEvent.setup();
  const client = createQueryClient({ retry: false });
  return {
    user,
    ...render(
      <I18nProvider>
        <QueryClientProvider client={client}>
          <TooltipProvider>{ui}</TooltipProvider>
        </QueryClientProvider>
      </I18nProvider>,
    ),
  };
};
