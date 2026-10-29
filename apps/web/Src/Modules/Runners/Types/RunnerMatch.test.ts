import { describe, expect, it } from 'vitest';

import {
  canTakeJob,
  candidatesFor,
  closestRunner,
  runnerFit,
  type IJobNeeds,
  type TRunnerMatchFields,
} from '@/Modules/Runners/Types/RunnerMatch';

const runner = (over: Partial<TRunnerMatchFields> = {}): TRunnerMatchFields => ({
  Id: 'r1',
  Labels: [],
  CapabilityLabels: ['dhow.os=linux', 'dhow.arch=amd64', 'dhow.isolation=container'],
  IsolationLevels: ['container'],
  State: 'active',
  ...over,
});

const needs = (labels: string[] = [], isolation: IJobNeeds['Isolation'] = 'container'): IJobNeeds => ({
  Labels: labels,
  Isolation: isolation,
});

describe('canTakeJob follows the server rule: labels ⊆ runner labels ∪ capability labels, isolation ∈ levels, state active', () => {
  it.each([
    ['a plain job on a plain runner', runner(), needs(), true],
    ['a runner without the label does not count', runner(), needs(['gpu']), false],
    ['a runner with the label', runner({ Labels: ['gpu'] }), needs(['gpu']), true],
    ['every label must be there', runner({ Labels: ['gpu'] }), needs(['gpu', 'arm64']), false],
    ['extra runner labels do not matter', runner({ Labels: ['gpu', 'arm64', 'eu'] }), needs(['arm64']), true],
    ['capability labels count', runner(), needs(['dhow.os=linux']), true],
    ['labels and capability labels together', runner({ Labels: ['gpu'] }), needs(['gpu', 'dhow.arch=amd64']), true],
    ['a capability the runner did not report', runner(), needs(['dhow.arch=arm64']), false],
    ['the isolation must be supported', runner({ Labels: ['gpu'] }), needs(['gpu'], 'gvisor'), false],
    ['a runner with both isolation levels', runner({ IsolationLevels: ['container', 'gvisor'] }), needs([], 'gvisor'), true],
    ['a paused runner does not count', runner({ Labels: ['gpu'], State: 'disabled' }), needs(['gpu']), false],
    ['a revoked runner does not count', runner({ Labels: ['gpu'], State: 'revoked' }), needs(['gpu']), false],
  ])('%s', (_, item, job, want) => {
    expect(canTakeJob(item, job)).toBe(want);
  });
});

describe('runnerFit', () => {
  it.each([
    ['canTake', runner({ Labels: ['arm64'] }), needs(['arm64']), [], true],
    ['missingLabels', runner({ Labels: ['linux'] }), needs(['arm64', 'docker', 'linux']), ['arm64', 'docker'], true],
    ['missingLabels', runner({ Labels: ['arm64'], State: 'disabled' }), needs(['arm64', 'gpu']), ['gpu'], true],
    ['missingLabels', runner(), needs(['gpu'], 'gvisor'), ['gpu'], false],
    ['noIsolation', runner({ Labels: ['gpu'] }), needs(['gpu'], 'gvisor'), [], false],
    ['paused', runner({ Labels: ['gpu'], State: 'disabled' }), needs(['gpu']), [], true],
  ] as const)('says %s', (verdict, item, job, missing, isolation) => {
    expect(runnerFit(item, job)).toEqual({
      Verdict: verdict,
      Missing: missing,
      Isolation: isolation,
    });
  });

  it('lists a label the job repeats only once', () => {
    expect(runnerFit(runner(), needs(['gpu', 'gpu'])).Missing).toEqual(['gpu']);
  });
});

describe('the runners the table shows and the closest one', () => {
  const runners = [
    runner({ Id: 'paused', Labels: ['gpu'], State: 'disabled' }),
    runner({ Id: 'revoked', Labels: ['gpu', 'arm64'], State: 'revoked' }),
    runner({ Id: 'two-short', Labels: [] }),
    runner({ Id: 'one-short', Labels: ['gpu'] }),
    runner({ Id: 'no-gvisor', Labels: ['gpu', 'arm64'], IsolationLevels: [] }),
  ];

  it('leaves the revoked runners out', () => {
    expect(candidatesFor(runners).map((item) => item.Id)).toEqual(['paused', 'two-short', 'one-short', 'no-gvisor']);
  });

  it('picks the runner that lacks the fewest labels and supports the isolation', () => {
    expect(closestRunner(runners, needs(['gpu', 'arm64']))?.Id).toBe('paused');
    expect(closestRunner(runners.slice(2), needs(['gpu', 'arm64']))?.Id).toBe('one-short');
  });

  it('has no closest runner when none lacks only labels', () => {
    expect(closestRunner(runners, needs([]))).toBeUndefined();
    expect(closestRunner([runner({ IsolationLevels: [] })], needs(['gpu']))).toBeUndefined();
  });
});
