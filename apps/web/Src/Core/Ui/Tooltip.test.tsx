import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { renderUi } from '@/Core/Testing/Render';
import { Tooltip } from '@/Core/Ui/Tooltip';

describe('Tooltip', () => {
  it('opens on keyboard focus and closes with Escape', async () => {
    const { user } = renderUi(
      <Tooltip label="System status">
        <button type="button">Trigger</button>
      </Tooltip>,
    );
    await user.tab();
    expect(await screen.findByRole('tooltip')).toHaveTextContent('System status');
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it('opens on hover', async () => {
    const { user } = renderUi(
      <Tooltip label="System status" side="end">
        <button type="button">Trigger</button>
      </Tooltip>,
    );
    await user.hover(screen.getByRole('button', { name: 'Trigger' }));
    expect(await screen.findByRole('tooltip')).toHaveTextContent('System status');
  });
});

describe('Tooltip on a menu with visible labels', () => {
  it('can be limited to phones, where the label is hidden', async () => {
    const { user } = renderUi(
      <Tooltip label="System status" phoneOnly>
        <button type="button">Trigger</button>
      </Tooltip>,
    );
    await user.tab();
    expect(await screen.findByRole('tooltip')).toHaveClass('md:hidden');
  });
});
