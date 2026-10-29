import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders } from '@/Core/Testing/Render';
import { Ar } from '@/I18n/Locales/Ar';
import type { TMessageKey } from '@/I18n/Messages';
import type { TJobState } from '@/Modules/Jobs/Api/JobsApi';
import { JobFilter } from '@/Modules/Jobs/Components/JobFilter';

const open = (value: readonly TJobState[]) => {
  const onChange = vi.fn();
  renderWithProviders(<JobFilter value={value} onChange={onChange} />);
  return { onChange, user: userEvent.setup() };
};

const chip = (name: string) => screen.getByRole('button', { name });

describe('JobFilter', () => {
  it('presses every state in the filter, and "all" only when there is none', () => {
    open(['failed', 'timed_out']);
    expect(chip(Ar['jobs.state.failed'])).toHaveAttribute('aria-pressed', 'true');
    expect(chip(Ar['jobs.state.timed_out'])).toHaveAttribute('aria-pressed', 'true');
    expect(chip(Ar['jobs.state.queued'])).toHaveAttribute('aria-pressed', 'false');
    expect(chip(Ar['jobs.filter.all'])).toHaveAttribute('aria-pressed', 'false');
  });

  it.each<[readonly TJobState[], TMessageKey, TJobState[]]>([
    [[], 'jobs.state.failed', ['failed']],
    [['failed'], 'jobs.state.timed_out', ['failed', 'timed_out']],
    [['timed_out'], 'jobs.state.failed', ['failed', 'timed_out']],
    [['failed', 'timed_out'], 'jobs.state.failed', ['timed_out']],
    [['failed', 'timed_out'], 'jobs.filter.all', []],
  ])('from %j, a click on %s gives %j', async (value, key, want) => {
    const { onChange, user } = open(value);
    await user.click(chip(Ar[key]));
    expect(onChange).toHaveBeenCalledWith(want);
  });
});
