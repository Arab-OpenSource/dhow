import { useState } from 'react';

import { isApiError } from '@/Core/Api/ApiError';
import { canManageRunners, canReadRunners } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Pager } from '@/Core/Ui/Pager';
import { Rows } from '@/Core/Ui/Row';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { RUNNERS_PAGE_SIZE } from '@/Modules/Runners/Api/RunnersApi';
import { CopyBlock } from '@/Modules/Runners/Components/CopyBlock';
import { RegistrationTokensCard } from '@/Modules/Runners/Components/RegistrationTokensCard';
import { RunnerRow } from '@/Modules/Runners/Components/RunnerRow';
import { useRunners } from '@/Modules/Runners/Hooks/useRunners';
import { registerCommandFor } from '@/Modules/Runners/Types/RunnerView';

export interface IRunnersBodyProps {
  onOpenRunner: (runnerId: string) => void;
  onAddRunner: () => void;
}

interface IContentProps extends IRunnersBodyProps {
  organizationId: string;
  manager: boolean;
}

const Forbidden = () => {
  const t = useT();
  return (
    <Card>
      <EmptyState icon="shield" text={t('runners.forbidden')} />
    </Card>
  );
};

const FirstRunner = ({ manager, onAddRunner }: { manager: boolean; onAddRunner: () => void }) => {
  const t = useT();
  return (
    <Card padded>
      <div className="grid gap-4">
        <EmptyState
          icon="server"
          text={manager ? t('runners.empty.manager') : t('runners.empty.member')}
          className="pb-2"
          action={
            manager ? (
              <Button iconStart="plus" onClick={onAddRunner}>
                {t('runners.add')}
              </Button>
            ) : null
          }
        />
        <ol className="m-0 grid gap-2 ps-5 text-body-sm text-secondary">
          <li>{t('runners.steps.token')}</li>
          <li className="grid gap-2">
            {t('runners.steps.command')}
            <CopyBlock
              id="register-command"
              label={t('runners.command.label')}
              value={registerCommandFor(window.location.origin)}
              copy={t('runners.command.copy')}
              copied={t('runners.command.copied')}
            />
          </li>
          <li>{t('runners.steps.run')}</li>
        </ol>
      </div>
    </Card>
  );
};

const RunnersContent = ({ organizationId, manager, onOpenRunner, onAddRunner }: IContentProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const [Page, setPage] = useState(1);
  const runners = useRunners(organizationId, Page);

  if (isApiError(runners.error) && runners.error.Status === 403) return <Forbidden />;
  if (runners.isError) {
    return (
      <div className="grid justify-items-start gap-3">
        <InlineAlert accent="failed" title={t('runners.error')}>
          {errorText(runners.error)}
        </InlineAlert>
        <Button
          size="sm"
          onClick={() => {
            void runners.refetch();
          }}
        >
          {t('runners.retry')}
        </Button>
      </div>
    );
  }
  if (runners.data === undefined) return <Skeleton label={t('runners.loading')} lines={5} />;

  const { Items, Meta } = runners.data;
  return (
    <>
      {Items.length === 0 ? (
        <FirstRunner manager={manager} onAddRunner={onAddRunner} />
      ) : (
        <Card title={t('runners.list.title')} icon="server" end={t('runners.list.total', { count: Meta.Total })}>
          <Rows label={t('runners.list.title')}>
            {Items.map((runner) => (
              <RunnerRow
                key={runner.Id}
                runner={runner}
                onOpen={() => {
                  onOpenRunner(runner.Id);
                }}
              />
            ))}
          </Rows>
        </Card>
      )}
      <Pager page={Meta.Page} limit={Meta.Limit || RUNNERS_PAGE_SIZE} total={Meta.Total} busy={runners.isFetching} onPage={setPage} />
      {manager ? <RegistrationTokensCard organizationId={organizationId} /> : null}
    </>
  );
};

export const RunnersBody = (props: IRunnersBodyProps) => {
  const membership = useCurrentMembership();
  if (membership === undefined || !canReadRunners(membership.Role)) return <Forbidden />;
  return <RunnersContent {...props} organizationId={membership.OrganizationId} manager={canManageRunners(membership.Role)} />;
};

export const RunnersFooter = ({ onAddRunner }: Pick<IRunnersBodyProps, 'onAddRunner'>) => {
  const t = useT();
  const role = useCurrentMembership()?.Role;
  if (!canManageRunners(role)) return null;
  return (
    <>
      <span className="flex-1" />
      <Button variant="primary" iconStart="plus" onClick={onAddRunner}>
        {t('runners.add')}
      </Button>
    </>
  );
};
