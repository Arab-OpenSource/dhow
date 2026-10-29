import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { renderUi } from '@/Core/Testing/Render';
import { CardStates, cardStateOf, type TCardState } from '@/Core/Ui/CardStates';

const view = (state: TCardState, onRetry = vi.fn()) =>
  renderUi(
    <CardStates
      state={state}
      loadingLabel="Loading jobs"
      errorTitle="Could not load"
      errorText="The server is down"
      retryLabel="Try again"
      onRetry={onRetry}
      emptyIcon="play"
      emptyText="Nothing yet"
    >
      <p>Loaded rows</p>
    </CardStates>,
  );

describe('CardStates', () => {
  it('shows a labelled skeleton while loading', () => {
    view('loading');
    expect(screen.getByRole('status', { name: 'Loading jobs' })).toBeInTheDocument();
    expect(screen.queryByText('Loaded rows')).not.toBeInTheDocument();
  });

  it('shows the error with a retry button', async () => {
    const onRetry = vi.fn();
    const { user } = view('error', onRetry);
    expect(screen.getByText('Could not load')).toBeInTheDocument();
    expect(screen.getByText('The server is down')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Try again' }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it('shows the empty text', () => {
    view('empty');
    expect(screen.getByText('Nothing yet')).toBeInTheDocument();
    expect(screen.queryByText('Loaded rows')).not.toBeInTheDocument();
  });

  it('shows the content once it is ready', () => {
    view('ready');
    expect(screen.getByText('Loaded rows')).toBeInTheDocument();
  });

  it('derives the state from a query', () => {
    expect(cardStateOf({ data: undefined, isError: false }, true)).toBe('loading');
    expect(cardStateOf({ data: undefined, isError: true }, true)).toBe('error');
    expect(cardStateOf({ data: { Items: [] }, isError: true }, true)).toBe('error');
    expect(cardStateOf({ data: { Items: [] }, isError: false }, true)).toBe('empty');
    expect(cardStateOf({ data: { Items: [1] }, isError: false }, false)).toBe('ready');
  });
});
