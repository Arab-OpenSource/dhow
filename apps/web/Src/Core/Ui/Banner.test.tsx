import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { Banner } from '@/Core/Ui/Banner';

describe('Banner', () => {
  it('announces failures as alerts and everything else as status', () => {
    const { rerender } = render(<Banner accent="failed" title="Server unreachable" />);
    expect(screen.getByRole('alert')).toHaveTextContent('Server unreachable');
    rerender(<Banner accent="awaiting" title="Setup pending" />);
    expect(screen.getByRole('status')).toHaveTextContent('Setup pending');
  });

  it('renders the body, the action and a dismiss button that works', async () => {
    const onDismiss = vi.fn();
    render(
      <Banner
        title="Setup pending"
        body="Finish it first"
        action={<button type="button">Retry</button>}
        dismissLabel="Dismiss"
        onDismiss={onDismiss}
      />,
    );
    expect(screen.getByText('Finish it first')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Dismiss' }));
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it('lets the action wrap under the text only when asked', () => {
    const { rerender } = render(<Banner title="Stuck" action={<button type="button">Fix</button>} />);
    expect(screen.getByRole('status')).not.toHaveClass('flex-wrap');
    rerender(<Banner wrap title="Stuck" action={<button type="button">Fix</button>} />);
    expect(screen.getByRole('status')).toHaveClass('flex-wrap');
  });
});
