import { describe, expect, it } from 'vitest';

import { jobsSearchOf, jobStatesOf, parseJobsSearch } from '@/App/Navigation/JobsSearch';

describe('the jobs search', () => {
  it.each([
    [{ state: 'failed' }, { state: 'failed' }],
    [{ state: 'failed,timed_out' }, { state: 'failed,timed_out' }],
    [{ state: 'timed_out, failed,failed' }, { state: 'failed,timed_out' }],
    [{ state: 'failed,paused' }, { state: 'failed' }],
    [{ state: 'paused' }, {}],
    [{ state: 3 }, {}],
    [{}, {}],
  ])('reads %j as %j', (search, want) => {
    expect(parseJobsSearch(search)).toEqual(want);
  });

  it('turns a list of states into one search value and back', () => {
    expect(jobsSearchOf(['timed_out', 'failed'])).toEqual({ state: 'failed,timed_out' });
    expect(jobsSearchOf([])).toEqual({});
    expect(jobStatesOf('failed,timed_out')).toEqual(['failed', 'timed_out']);
    expect(jobStatesOf(undefined)).toEqual([]);
  });
});
