import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { InlineAlert } from '@/Core/Ui/InlineAlert';

describe('InlineAlert', () => {
  it('is a note unless it reports a failure', () => {
    const { rerender } = render(<InlineAlert title="Heads up">Detail text</InlineAlert>);
    expect(screen.getByRole('note')).toHaveTextContent('Detail text');
    rerender(<InlineAlert accent="failed" title="Could not load" />);
    expect(screen.getByRole('alert')).toHaveTextContent('Could not load');
  });
});
