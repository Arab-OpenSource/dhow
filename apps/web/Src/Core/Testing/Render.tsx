import { render, type RenderResult } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ReactElement } from 'react';

import { TooltipProvider } from '@/Core/Ui/Tooltip';

export interface IUiRender extends RenderResult {
  user: ReturnType<typeof userEvent.setup>;
}

export const renderUi = (ui: ReactElement): IUiRender => {
  const user = userEvent.setup();
  return { user, ...render(<TooltipProvider>{ui}</TooltipProvider>) };
};
