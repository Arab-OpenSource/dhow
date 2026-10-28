import { isApiError } from '@/Core/Api/ApiError';
import { canManageRunners, canReadRunners } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { Icon } from '@/Core/Ui/Icon';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import type { TMessageKey } from '@/I18n/Messages';
import type { TRunner } from '@/Modules/Runners/Api/RunnersApi';
import { RunnerEditCard, RunnerPauseCard, RunnerRevokeCard } from '@/Modules/Runners/Components/RunnerActions';
import { RunnerFacts } from '@/Modules/Runners/Components/RunnerFacts';
import { useRunner } from '@/Modules/Runners/Hooks/useRunners';
import { VIEW_LABEL, VIEW_TONE, viewOf, type TRunnerView } from '@/Modules/Runners/Types/RunnerView';

export interface IRunnerBodyProps {
  runnerId: string;
}

const ABOUT: Record<TRunnerView, TMessageKey> = {
  online: 'runners.about.online',
  offline: 'runners.about.offline',
  paused: 'runners.about.paused',
  cloneSuspected: 'runners.about.cloneSuspected',
  revoked: 'runners.about.revoked',
};

const Header = ({ runner }: { runner: TRunner }) => {
  const t = useT();
  const view = viewOf(runner);
  return (
    <Card padded>
      <div className="flex min-w-0 items-start gap-3">
        <Icon name="server" size={22} className="mt-0.5 flex-none text-secondary" />
        <div className="grid min-w-0 flex-1 gap-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="m-0 min-w-0 truncate text-title-sm">
              <bdi>{runner.Name}</bdi>
            </h3>
            <Badge tone={VIEW_TONE[view]}>{t(VIEW_LABEL[view])}</Badge>
          </div>
          <p className="m-0 text-body-sm text-secondary">{t(ABOUT[view])}</p>
        </div>
      </div>
    </Card>
  );
};

const Loaded = ({ organizationId, runnerId, manager }: IRunnerBodyProps & { organizationId: string; manager: boolean }) => {
  const t = useT();
  const errorText = useApiErrorText();
  const runner = useRunner(organizationId, runnerId);

  if (isApiError(runner.error) && runner.error.Status === 404) {
    return (
      <Card>
        <EmptyState icon="server" text={t('runners.detail.missing')} />
      </Card>
    );
  }
  if (runner.isError) {
    return (
      <div className="grid justify-items-start gap-3">
        <InlineAlert accent="failed" title={t('runners.detail.error')}>
          {errorText(runner.error)}
        </InlineAlert>
        <Button
          size="sm"
          onClick={() => {
            void runner.refetch();
          }}
        >
          {t('runners.retry')}
        </Button>
      </div>
    );
  }
  if (runner.data === undefined) return <Skeleton label={t('runners.loading')} lines={6} />;

  const live = runner.data.State !== 'revoked';
  return (
    <>
      <Header runner={runner.data} />
      <RunnerFacts runner={runner.data} />
      {manager && live ? (
        <>
          <RunnerEditCard key={`${runner.data.Name}|${runner.data.Labels.join(',')}`} organizationId={organizationId} runner={runner.data} />
          <RunnerPauseCard organizationId={organizationId} runner={runner.data} />
          <RunnerRevokeCard organizationId={organizationId} runner={runner.data} />
        </>
      ) : null}
      {!manager && live ? <p className="m-0 text-body-sm text-tertiary">{t('runners.detail.readOnly')}</p> : null}
    </>
  );
};

export const RunnerBody = ({ runnerId }: IRunnerBodyProps) => {
  const t = useT();
  const membership = useCurrentMembership();
  if (membership === undefined || !canReadRunners(membership.Role)) {
    return (
      <Card>
        <EmptyState icon="shield" text={t('runners.forbidden')} />
      </Card>
    );
  }
  return <Loaded organizationId={membership.OrganizationId} runnerId={runnerId} manager={canManageRunners(membership.Role)} />;
};
