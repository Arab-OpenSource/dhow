import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { renderUi } from '@/Core/Testing/Render';
import { StatTile } from '@/Core/Ui/StatTile';

describe('StatTile', () => {
  it('is a real link whose name carries the number, the label and the note', () => {
    renderUi(<StatTile value={3} label="Queued" sub="2 with no runner" icon="queue" tone="queue" href="/jobs?state=queued" onSelect={vi.fn()} />);
    const link = screen.getByRole('link', { name: /3\s*Queued/ });
    expect(link).toHaveAttribute('href', '/jobs?state=queued');
    expect(link).toHaveAttribute('data-tone', 'queue');
    expect(link).toHaveTextContent('2 with no runner');
    expect(screen.getByText('3')).toHaveClass('text-stat');
  });

  it('opens in the app on a plain click and leaves modified clicks to the browser', () => {
    const onSelect = vi.fn();
    renderUi(<StatTile value={0} label="Failed" icon="alert" tone="bad" href="/jobs?state=failed" onSelect={onSelect} />);
    const link = screen.getByRole('link', { name: /Failed/ });
    fireEvent.click(link);
    fireEvent.click(link, { ctrlKey: true });
    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it('leaves the note out when there is none', () => {
    renderUi(<StatTile value={1} label="Running" icon="play" tone="info" href="/jobs" onSelect={vi.fn()} />);
    expect(screen.getByRole('link').textContent).toBe('1Running');
  });
});
