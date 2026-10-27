import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { EmptyState } from '@/Core/Ui/EmptyState';

describe('EmptyState', () => {
  it('shows one sentence and its action', () => {
    render(<EmptyState icon="project" text="Nothing here yet" action={<button type="button">Do it</button>} />);
    expect(screen.getByText('Nothing here yet')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Do it' })).toBeInTheDocument();
  });
});
