import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Badge, type TBadgeTone } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { CardStates, cardStateOf } from '@/Core/Ui/CardStates';
import { Icon } from '@/Core/Ui/Icon';
import { StatusDot } from '@/Core/Ui/StatusDot';
import type { TMessageKey } from '@/I18n/Messages';
import { useT } from '@/I18n/useT';
import type { TRunner } from '@/Modules/Runners/Api/RunnersApi';
import { useRunnerAccess } from '@/Modules/Runners/Hooks/useRunnerAccess';
import { useAllRunners } from '@/Modules/Runners/Hooks/useRunners';
import { candidatesFor, closestRunner, runnerFit, type IJobNeeds, type TRunnerVerdict } from '@/Modules/Runners/Types/RunnerMatch';
import { VIEW_DOT, VIEW_LABEL, viewOf } from '@/Modules/Runners/Types/RunnerView';

export interface IWhyWaitingCardProps {
  needs: IJobNeeds;
  onAddRunner: () => void;
  onOpenRunner: (runnerId: string) => void;
}

interface ILoadedProps extends IWhyWaitingCardProps {
  organizationId: string;
  manager: boolean;
}

const VERDICT_TONE: Record<TRunnerVerdict, TBadgeTone> = {
  canTake: 'ok',
  missingLabels: 'bad',
  noIsolation: 'bad',
  paused: 'warn',
};

const VERDICT_LABEL: Record<TRunnerVerdict, TMessageKey> = {
  canTake: 'runners.fit.verdict.canTake',
  missingLabels: 'runners.fit.verdict.missingLabels',
  noIsolation: 'runners.fit.verdict.noIsolation',
  paused: 'runners.fit.verdict.paused',
};

const CELL = 'px-card py-3 align-middle max-md:flex max-md:flex-wrap max-md:items-center max-md:gap-x-2 max-md:gap-y-1 max-md:p-0';
const LABELLED = `${CELL} max-md:before:text-body-sm max-md:before:text-tertiary max-md:before:content-[attr(data-label)]`;
const HEAD = 'px-card pb-2 text-start text-body-sm font-normal text-tertiary';

const FitRow = ({ runner, needs }: { runner: TRunner; needs: IJobNeeds }) => {
  const t = useT();
  const fit = runnerFit(runner, needs);
  const view = viewOf(runner);
  return (
    <tr className="border-t border-line max-md:grid max-md:gap-2 max-md:px-card max-md:py-3">
      <th scope="row" className={`${CELL} text-start text-label font-normal`}>
        <bdi dir="ltr" className="font-mono">
          {runner.Name}
        </bdi>
      </th>
      <td data-label={t('runners.fit.col.state')} className={LABELLED}>
        <StatusDot status={VIEW_DOT[view]}>{t(VIEW_LABEL[view])}</StatusDot>
      </td>
      <td data-label={t('runners.fit.col.isolation')} className={LABELLED}>
        <span
          className={
            fit.Isolation
              ? 'inline-flex items-center gap-1.5 text-body-sm text-secondary'
              : 'inline-flex items-center gap-1.5 text-body-sm text-bad-fg'
          }
        >
          <Icon name={fit.Isolation ? 'check' : 'close'} size={15} />
          {fit.Isolation ? t('runners.fit.isolation.yes') : t('runners.fit.isolation.no')}
        </span>
      </td>
      <td data-label={t('runners.fit.col.missing')} className={LABELLED}>
        {fit.Missing.length === 0 ? (
          <span className="text-body-sm text-tertiary">{t('runners.fit.missing.none')}</span>
        ) : (
          <span className="inline-flex flex-wrap gap-1.5">
            {fit.Missing.map((label) => (
              <Badge key={label} tone="warn">
                <bdi dir="ltr" className="font-mono">
                  {label}
                </bdi>
              </Badge>
            ))}
          </span>
        )}
      </td>
      <td data-label={t('runners.fit.col.verdict')} className={LABELLED}>
        <Badge tone={VERDICT_TONE[fit.Verdict]}>{t(VERDICT_LABEL[fit.Verdict])}</Badge>
      </td>
    </tr>
  );
};

const FitAction = ({ runners, needs, manager, onAddRunner, onOpenRunner }: ILoadedProps & { runners: TRunner[] }) => {
  const t = useT();
  if (!manager) return <p className="m-0 text-body-sm text-secondary">{t('runners.fit.ask')}</p>;
  const paused = runners.find((runner) => runnerFit(runner, needs).Verdict === 'paused');
  if (paused !== undefined) {
    return (
      <Button
        variant="primary"
        className="justify-self-start"
        onClick={() => {
          onOpenRunner(paused.Id);
        }}
      >
        {t('runners.fit.resume', { name: paused.Name })}
      </Button>
    );
  }
  const closest = closestRunner(runners, needs);
  if (closest !== undefined) {
    return (
      <Button
        variant="primary"
        iconStart="settings"
        className="justify-self-start"
        onClick={() => {
          onOpenRunner(closest.Id);
        }}
      >
        {t('runners.fit.edit', { name: closest.Name })}
      </Button>
    );
  }
  return (
    <Button variant="primary" iconStart="plus" className="justify-self-start" onClick={onAddRunner}>
      {t('runners.add')}
    </Button>
  );
};

const Loaded = (props: ILoadedProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const query = useAllRunners(props.organizationId);
  const runners = candidatesFor(query.data?.Items ?? []);

  return (
    <Card title={t('runners.fit.title')} icon="queue">
      <p className="m-0 px-card pb-3 text-body-sm text-secondary">{t('runners.fit.lead')}</p>
      <CardStates
        state={cardStateOf(query, runners.length === 0)}
        loadingLabel={t('runners.loading')}
        errorTitle={t('runners.error')}
        errorText={query.error ? errorText(query.error) : undefined}
        retryLabel={t('runners.retry')}
        onRetry={() => {
          void query.refetch();
        }}
        emptyIcon="server"
        emptyText={props.manager ? t('runners.fit.empty.manager') : t('runners.fit.empty.member')}
        emptyAction={
          props.manager ? (
            <Button variant="primary" iconStart="plus" onClick={props.onAddRunner}>
              {t('runners.add')}
            </Button>
          ) : undefined
        }
      >
        <table aria-label={t('runners.fit.table')} className="w-full border-collapse max-md:block">
          <thead className="max-md:sr-only">
            <tr>
              <th scope="col" className={HEAD}>
                {t('runners.fit.col.name')}
              </th>
              <th scope="col" className={HEAD}>
                {t('runners.fit.col.state')}
              </th>
              <th scope="col" className={HEAD}>
                {t('runners.fit.col.isolation')}
              </th>
              <th scope="col" className={HEAD}>
                {t('runners.fit.col.missing')}
              </th>
              <th scope="col" className={HEAD}>
                {t('runners.fit.col.verdict')}
              </th>
            </tr>
          </thead>
          <tbody className="max-md:block">
            {runners.map((runner) => (
              <FitRow key={runner.Id} runner={runner} needs={props.needs} />
            ))}
          </tbody>
        </table>
        <div className="grid gap-2 border-t border-line px-card py-card">
          <FitAction {...props} runners={runners} />
        </div>
      </CardStates>
    </Card>
  );
};

export const WhyWaitingCard = (props: IWhyWaitingCardProps) => {
  const access = useRunnerAccess();
  if (access.Kind !== 'allowed') return null;
  return <Loaded {...props} organizationId={access.OrganizationId} manager={access.Manager} />;
};
