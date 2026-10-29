export type TOverviewTarget = 'running' | 'queued' | 'stuck' | 'failed' | 'runners' | 'offline' | 'invitations' | 'checks' | 'addRunner';

export interface IOverviewLinks {
  href: (target: TOverviewTarget) => string;
  open: (target: TOverviewTarget) => void;
}

export type TCountForm = 'one' | 'two' | 'few' | 'many';

const FEW_MAX = 10;

export const countForm = (count: number): TCountForm => {
  if (count === 1) return 'one';
  if (count === 2) return 'two';
  return count <= FEW_MAX ? 'few' : 'many';
};
