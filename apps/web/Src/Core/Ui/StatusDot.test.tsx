import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { StatusDot, type TStatus } from '@/Core/Ui/StatusDot';

const STATUSES: TStatus[] = ['live', 'building', 'awaiting', 'failed', 'queued', 'rolledback'];

describe('StatusDot', () => {
  it('always carries the state word next to the dot', () => {
    for (const status of STATUSES) {
      const { unmount } = render(<StatusDot status={status}>{`word-${status}`}</StatusDot>);
      expect(screen.getByText(`word-${status}`)).toHaveAttribute('data-status', status);
      unmount();
    }
  });

  it('hides the dot from assistive technology and pulses only while building', () => {
    const { container, rerender } = render(<StatusDot status="building">Building</StatusDot>);
    expect(container.querySelector('i')).toHaveAttribute('aria-hidden', 'true');
    expect(container.querySelector('i')).toHaveClass('animate-dhow-pulse');
    rerender(<StatusDot status="live">Live</StatusDot>);
    expect(container.querySelector('i')).not.toHaveClass('animate-dhow-pulse');
  });
});
