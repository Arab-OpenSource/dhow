import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { CardStates, cardStateOf } from '@/Core/Ui/CardStates';
import { Rows } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import { RunnerRow } from '@/Modules/Runners/Components/RunnerRow';
import { useRunnerSummary } from '@/Modules/Runners/Hooks/useRunners';

export interface IRunnersSummaryCardProps {
  organizationId: string;
  onOpenRunner: (runnerId: string) => void;
  onShowAll: () => void;
  onAddRunner?: () => void;
  className?: string;
}

export const RunnersSummaryCard = ({ organizationId, onOpenRunner, onShowAll, onAddRunner, className }: IRunnersSummaryCardProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const runners = useRunnerSummary(organizationId);
  const items = runners.data?.Items ?? [];

  return (
    <Card
      title={t('home.runners.title')}
      icon="server"
      className={className}
      end={
        items.length > 0 ? (
          <Button size="sm" variant="ghost" onClick={onShowAll}>
            {t('home.runners.all')}
          </Button>
        ) : null
      }
    >
      <CardStates
        state={cardStateOf(runners, items.length === 0)}
        loadingLabel={t('runners.loading')}
        errorTitle={t('runners.error')}
        errorText={runners.error ? errorText(runners.error) : undefined}
        retryLabel={t('jobs.retry')}
        onRetry={() => {
          void runners.refetch();
        }}
        emptyIcon="server"
        emptyText={t('home.runners.empty')}
        emptyAction={
          onAddRunner ? (
            <Button size="sm" iconStart="plus" onClick={onAddRunner}>
              {t('home.runners.add')}
            </Button>
          ) : undefined
        }
      >
        <Rows label={t('home.runners.title')}>
          {items.map((runner) => (
            <RunnerRow
              key={runner.Id}
              runner={runner}
              onOpen={() => {
                onOpenRunner(runner.Id);
              }}
            />
          ))}
        </Rows>
      </CardStates>
    </Card>
  );
};
