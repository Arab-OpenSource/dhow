import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { renderUi } from '@/Core/Testing/Render';
import { IconButton } from '@/Core/Ui/IconButton';

describe('IconButton', () => {
  it('is named by its label and fires onClick', async () => {
    const onClick = vi.fn();
    const { user } = renderUi(<IconButton icon="bell" label="Open notifications" onClick={onClick} />);
    await user.click(screen.getByRole('button', { name: 'Open notifications' }));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('shows the unread dot without changing the accessible name', () => {
    const { container } = renderUi(<IconButton icon="bell" label="Open notifications" badge />);
    expect(screen.getByRole('button', { name: 'Open notifications' })).toBeInTheDocument();
    expect(container.querySelector('i')).toHaveAttribute('aria-hidden', 'true');
  });

  it('drops the edge outline when plain', () => {
    renderUi(<IconButton icon="close" label="Close" plain />);
    expect(screen.getByRole('button', { name: 'Close' })).not.toHaveClass('edge');
  });
});
