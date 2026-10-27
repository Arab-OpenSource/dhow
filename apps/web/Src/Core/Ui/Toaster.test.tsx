import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { useToastStore } from '@/Core/Store/useToastStore';
import { Toaster } from '@/Core/Ui/Toaster';

afterEach(() => {
  vi.useRealTimers();
  useToastStore.setState({ Toasts: [] });
});

describe('Toaster', () => {
  it('announces a pushed toast as status and dismisses it on request', async () => {
    render(<Toaster dismissLabel="Dismiss" />);
    act(() => {
      useToastStore.getState().Push({ Id: 'a', Accent: 'failed', Title: 'That link is gone', Detail: 'Closed it' });
    });
    expect(screen.getByRole('status')).toHaveTextContent('That link is gone');
    expect(screen.getByText('Closed it')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Dismiss' }));
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('does not stack the same toast twice', () => {
    render(<Toaster dismissLabel="Dismiss" />);
    act(() => {
      useToastStore.getState().Push({ Id: 'a', Accent: 'live', Title: 'Once' });
      useToastStore.getState().Push({ Id: 'a', Accent: 'live', Title: 'Once' });
    });
    expect(screen.getAllByRole('status')).toHaveLength(1);
  });

  it('removes a toast by itself after a few seconds', () => {
    vi.useFakeTimers();
    render(<Toaster dismissLabel="Dismiss" />);
    act(() => {
      useToastStore.getState().Push({ Id: 'a', Accent: 'live', Title: 'Saved' });
    });
    expect(screen.getByRole('status')).toBeInTheDocument();
    act(() => {
      vi.advanceTimersByTime(6100);
    });
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });
});
