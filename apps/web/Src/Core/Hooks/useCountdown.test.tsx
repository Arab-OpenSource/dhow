import { act, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useCountdown } from '@/Core/Hooks/useCountdown';

const Probe = ({ seconds }: { seconds: number }) => {
  const { Remaining, Start } = useCountdown();
  return (
    <div>
      <output>{Remaining}</output>
      <button
        type="button"
        onClick={() => {
          Start(seconds);
        }}
      >
        go
      </button>
    </div>
  );
};

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useCountdown', () => {
  it('counts down once a second and stops at zero', () => {
    render(<Probe seconds={3} />);
    expect(screen.getByRole('status')).toHaveTextContent('0');
    act(() => {
      screen.getByRole('button').click();
    });
    expect(screen.getByRole('status')).toHaveTextContent('3');
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(screen.getByRole('status')).toHaveTextContent('2');
    act(() => {
      vi.advanceTimersByTime(5000);
    });
    expect(screen.getByRole('status')).toHaveTextContent('0');
  });

  it('restarts from the new length', () => {
    render(<Probe seconds={10} />);
    act(() => {
      screen.getByRole('button').click();
    });
    act(() => {
      vi.advanceTimersByTime(4000);
    });
    expect(screen.getByRole('status')).toHaveTextContent('6');
    act(() => {
      screen.getByRole('button').click();
    });
    expect(screen.getByRole('status')).toHaveTextContent('10');
  });
});
