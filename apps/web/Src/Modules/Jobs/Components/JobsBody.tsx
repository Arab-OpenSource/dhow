import { useState } from 'react';

import { isApiError } from '@/Core/Api/ApiError';
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
import type { TJobState } from '@/Modules/Jobs/Api/JobsApi';
import { JobFilter } from '@/Modules/Jobs/Components/JobFilter';
import { JobRow } from '@/Modules/Jobs/Components/JobRow';
import { JOBS_PAGE_SIZE, useJobs, useWaitingJobs } from '@/Modules/Jobs/Hooks/useJobs';
import { useNow } from '@/Modules/Jobs/Hooks/useNow';
import { isFinalState, isStuck } from '@/Modules/Jobs/Types/JobStates';

export interface IJobsBodyProps {
  onOpenJob: (jobId: string) => void;
}

const MINUTE_MS = 60_000;

const StuckAlert = ({ organizationId, onShow }: { organizationId: string; onShow: () => void }) => {
  const t = useT();
  const waiting = useWaitingJobs(organizationId);
  const stuck = waiting.data?.Items.filter(isStuck).length ?? 0;
  if (stuck === 0) return null;
  return (
    <div className="grid justify-items-start gap-2">
      <InlineAlert accent="awaiting" title={stuck === 1 ? t('jobs.stuck.titleOne') : t('jobs.stuck.title', { count: stuck })}>
        {t('jobs.stuck.about')}
      </InlineAlert>
      <Button size="sm" iconStart="queue" onClick={onShow}>
        {t('jobs.stuck.show')}
      </Button>
    </div>
  );
};

const JobsContent = ({ organizationId, onOpenJob }: IJobsBodyProps & { organizationId: string }) => {
  const t = useT();
  const errorText = useApiErrorText();
  const [Page, setPage] = useState(1);
  const [Filter, setFilter] = useState<TJobState | null>(null);
  const jobs = useJobs(organizationId, Page, Filter);
  const items = jobs.data?.Items ?? [];
  const now = useNow(items.some((job) => !isFinalState(job.State)), MINUTE_MS / 4);

  const HandleFilter = (state: TJobState | null) => {
    setFilter(state);
    setPage(1);
  };

  return (
    <>
      <StuckAlert
        organizationId={organizationId}
        onShow={() => {
          HandleFilter('queued');
        }}
      />
      <JobFilter value={Filter} onChange={HandleFilter} />
      {jobs.isError ? (
        <div className="grid justify-items-start gap-3">
          <InlineAlert accent="failed" title={t('jobs.error.title')}>
            {isApiError(jobs.error) ? errorText(jobs.error) : null}
          </InlineAlert>
          <Button
            size="sm"
            onClick={() => {
              void jobs.refetch();
            }}
          >
            {t('jobs.retry')}
          </Button>
        </div>
      ) : null}
      {jobs.data === undefined && !jobs.isError ? <Skeleton label={t('jobs.loading')} lines={6} /> : null}
      {jobs.data !== undefined && items.length === 0 ? (
        <Card>
          {Filter === null ? (
            <div className="grid gap-1 px-5 pt-6 pb-2 text-center">
              <h3 className="m-0 text-title-card">{t('jobs.empty.title')}</h3>
            </div>
          ) : null}
          <EmptyState
            icon="play"
            className={Filter === null ? 'pt-2' : undefined}
            text={Filter === null ? t('jobs.empty.about', { request: `POST /api/v1/organizations/${organizationId}/jobs` }) : t('jobs.empty.filtered')}
            action={
              Filter === null ? undefined : (
                <Button
                  size="sm"
                  onClick={() => {
                    HandleFilter(null);
                  }}
                >
                  {t('jobs.empty.showAll')}
                </Button>
              )
            }
          />
        </Card>
      ) : null}
      {items.length > 0 ? (
        <Card title={t('jobs.list.title')} icon="play" end={t('jobs.list.total', { count: jobs.data?.Meta.Total ?? items.length })}>
          <Rows label={t('jobs.list.title')}>
            {items.map((job) => (
              <JobRow key={job.Id} job={job} now={now} onOpen={onOpenJob} />
            ))}
          </Rows>
        </Card>
      ) : null}
      {jobs.data ? (
        <Pager
          page={jobs.data.Meta.Page}
          limit={jobs.data.Meta.Limit || JOBS_PAGE_SIZE}
          total={jobs.data.Meta.Total}
          busy={jobs.isFetching}
          onPage={setPage}
        />
      ) : null}
    </>
  );
};

export const JobsBody = ({ onOpenJob }: IJobsBodyProps) => {
  const membership = useCurrentMembership();
  if (membership === undefined) return null;
  return <JobsContent organizationId={membership.OrganizationId} onOpenJob={onOpenJob} />;
};
