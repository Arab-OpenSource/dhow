import { screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { startBackend } from '@/Core/Testing/Backend';
import { ORG_ID } from '@/Core/Testing/Fetch';
import { gate, type TGateMode } from '@/Core/Testing/Gate';
import { jobsFixture, jobsRoute, type TJobFixture } from '@/Core/Testing/JobsBackend';
import { renderWithProviders } from '@/Core/Testing/Render';
import { Ar } from '@/I18n/Locales/Ar';
import { RecentJobsCard } from '@/Modules/Jobs/Components/RecentJobsCard';

const mount = (jobs: TJobFixture[], mode: TGateMode = 'pass') => {
  const jobsGate = gate(/\/jobs$/, mode);
  const backend = startBackend({ routes: [jobsGate.route, jobsRoute({ jobs })] });
  const onOpenJob = vi.fn();
  const onShowAll = vi.fn();
  const view = renderWithProviders(<RecentJobsCard organizationId={ORG_ID} onOpenJob={onOpenJob} onShowAll={onShowAll} />);
  return { ...view, ...backend, jobsGate, onOpenJob, onShowAll };
};

describe('RecentJobsCard', () => {
  it('shows a skeleton while the jobs load', async () => {
    mount(jobsFixture(2), 'hang');
    expect(await screen.findByRole('status', { name: Ar['jobs.loading'] })).toBeInTheDocument();
  });

  it('shows an error and loads again on retry', async () => {
    const { jobsGate, user } = mount(jobsFixture(2), 'fail');
    expect(await screen.findByText(Ar['jobs.error.title'])).toBeInTheDocument();
    jobsGate.set('pass');
    await user.click(screen.getByRole('button', { name: Ar['jobs.retry'] }));
    expect(await screen.findByRole('list', { name: Ar['home.jobs.title'] })).toBeInTheDocument();
  });

  it('says so when there are no jobs, without a link to an empty list', async () => {
    mount([]);
    expect(await screen.findByText(Ar['home.jobs.empty'])).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: Ar['home.jobs.all'] })).not.toBeInTheDocument();
  });

  it('lists the first five jobs and opens one or the whole list', async () => {
    const { calls, user, onOpenJob, onShowAll } = mount(jobsFixture(8));
    const list = await screen.findByRole('list', { name: Ar['home.jobs.title'] });
    expect(within(list).getAllByRole('listitem')).toHaveLength(5);
    const asked = calls.map((call) => new URL(call.url)).filter((url) => url.pathname.endsWith('/jobs'));
    expect(asked.map((url) => url.search)).toEqual(['?Page=1&Limit=5']);
    await user.click(within(list).getByRole('button', { name: /job-1\b/ }));
    expect(onOpenJob).toHaveBeenCalledWith('0198a000-0000-7000-8000-000000000000');
    await user.click(screen.getByRole('button', { name: Ar['home.jobs.all'] }));
    expect(onShowAll).toHaveBeenCalledOnce();
  });
});
