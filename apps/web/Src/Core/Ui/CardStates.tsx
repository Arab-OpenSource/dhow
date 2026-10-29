import type { ReactNode } from 'react';

import { Button } from '@/Core/Ui/Button';
import { EmptyState } from '@/Core/Ui/EmptyState';
import type { TIconName } from '@/Core/Ui/Icons';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';

export type TCardState = 'loading' | 'error' | 'empty' | 'ready';

export interface ICardStatesProps {
  state: TCardState;
  loadingLabel: string;
  errorTitle: string;
  errorText?: string;
  retryLabel: string;
  onRetry: () => void;
  emptyIcon: TIconName;
  emptyText: string;
  emptyAction?: ReactNode;
  lines?: number;
  children?: ReactNode;
}

export interface IQueryLike {
  data: unknown;
  isError: boolean;
}

export const cardStateOf = (query: IQueryLike, empty: boolean): TCardState => {
  if (query.isError) return 'error';
  if (query.data === undefined) return 'loading';
  return empty ? 'empty' : 'ready';
};

export const CardStates = ({
  state,
  loadingLabel,
  errorTitle,
  errorText,
  retryLabel,
  onRetry,
  emptyIcon,
  emptyText,
  emptyAction,
  lines = 3,
  children,
}: ICardStatesProps) => {
  if (state === 'loading') {
    return (
      <div className="px-card pt-2 pb-card">
        <Skeleton label={loadingLabel} lines={lines} />
      </div>
    );
  }
  if (state === 'error') {
    return (
      <div className="grid justify-items-start gap-3 px-card pt-1 pb-card">
        <InlineAlert accent="failed" title={errorTitle}>
          {errorText}
        </InlineAlert>
        <Button size="sm" onClick={onRetry}>
          {retryLabel}
        </Button>
      </div>
    );
  }
  if (state === 'empty') return <EmptyState icon={emptyIcon} text={emptyText} action={emptyAction} className="py-6" />;
  return <>{children}</>;
};
