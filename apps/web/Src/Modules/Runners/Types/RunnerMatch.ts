import type { TRunner, TRunnerIsolation } from '@/Modules/Runners/Api/RunnersApi';

export interface IJobNeeds {
  Labels: readonly string[];
  Isolation: TRunnerIsolation;
}

export type TRunnerMatchFields = Pick<TRunner, 'Id' | 'Labels' | 'CapabilityLabels' | 'IsolationLevels' | 'State'>;

export type TRunnerVerdict = 'canTake' | 'missingLabels' | 'noIsolation' | 'paused';

export interface IRunnerFit {
  Missing: string[];
  Isolation: boolean;
  Verdict: TRunnerVerdict;
}

export const missingLabels = (runner: TRunnerMatchFields, needs: IJobNeeds): string[] => {
  const has = new Set([...runner.Labels, ...runner.CapabilityLabels]);
  return [...new Set(needs.Labels)].filter((label) => !has.has(label));
};

export const supportsIsolation = (runner: TRunnerMatchFields, needs: IJobNeeds): boolean =>
  runner.IsolationLevels.includes(needs.Isolation);

export const canTakeJob = (runner: TRunnerMatchFields, needs: IJobNeeds): boolean =>
  runner.State === 'active' && missingLabels(runner, needs).length === 0 && supportsIsolation(runner, needs);

const verdictOf = (runner: TRunnerMatchFields, missing: readonly string[], isolation: boolean): TRunnerVerdict => {
  if (missing.length > 0) return 'missingLabels';
  if (!isolation) return 'noIsolation';
  return runner.State === 'active' ? 'canTake' : 'paused';
};

export const runnerFit = (runner: TRunnerMatchFields, needs: IJobNeeds): IRunnerFit => {
  const missing = missingLabels(runner, needs);
  const isolation = supportsIsolation(runner, needs);
  return {
    Missing: missing,
    Isolation: isolation,
    Verdict: verdictOf(runner, missing, isolation),
  };
};

export const candidatesFor = <TItem extends TRunnerMatchFields>(runners: readonly TItem[]): TItem[] =>
  runners.filter((runner) => runner.State !== 'revoked');

export const closestRunner = <TItem extends TRunnerMatchFields>(runners: readonly TItem[], needs: IJobNeeds): TItem | undefined => {
  let best: TItem | undefined;
  let fewest = Number.POSITIVE_INFINITY;
  for (const runner of candidatesFor(runners)) {
    const missing = missingLabels(runner, needs).length;
    if (missing > 0 && missing < fewest && supportsIsolation(runner, needs)) {
      best = runner;
      fewest = missing;
    }
  }
  return best;
};
