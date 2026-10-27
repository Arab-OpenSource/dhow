import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { Banner } from '@/Core/Ui/Banner';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';

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
});

describe('InlineAlert', () => {
  it('is a note unless it reports a failure', () => {
    const { rerender } = render(<InlineAlert title="Heads up">Detail text</InlineAlert>);
    expect(screen.getByRole('note')).toHaveTextContent('Detail text');
    rerender(<InlineAlert accent="failed" title="Could not load" />);
    expect(screen.getByRole('alert')).toHaveTextContent('Could not load');
  });
});

describe('Skeleton', () => {
  it('is a busy status region with the requested number of bars', () => {
    const { container } = render(<Skeleton label="Loading" lines={4} />);
    const region = screen.getByRole('status', { name: 'Loading' });
    expect(region).toHaveAttribute('aria-busy', 'true');
    expect(container.querySelectorAll('i')).toHaveLength(4);
  });
});

describe('EmptyState', () => {
  it('shows one sentence and its action', () => {
    render(<EmptyState icon="project" text="Nothing here yet" action={<button type="button">Do it</button>} />);
    expect(screen.getByText('Nothing here yet')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Do it' })).toBeInTheDocument();
  });
});
