import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { Skeleton } from '@/Core/Ui/Skeleton';

describe('Skeleton', () => {
  it('is a busy status region with the requested number of bars', () => {
    const { container } = render(<Skeleton label="Loading" lines={4} />);
    const region = screen.getByRole('status', { name: 'Loading' });
    expect(region).toHaveAttribute('aria-busy', 'true');
    expect(container.querySelectorAll('i')).toHaveLength(4);
  });
});
