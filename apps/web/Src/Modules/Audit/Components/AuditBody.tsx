import { useState } from 'react';

import { isApiError } from '@/Core/Api/ApiError';
import { canReadAudit } from '@/Core/Auth/AuthTypes';
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
import { AuditEventRow } from '@/Modules/Audit/Components/AuditEventRow';
import { AuditFilters } from '@/Modules/Audit/Components/AuditFilters';
import { VerifyCard } from '@/Modules/Audit/Components/VerifyCard';
import { AUDIT_PAGE_SIZE, useAuditEvents } from '@/Modules/Audit/Hooks/useAuditEvents';
import { useActors } from '@/Modules/Audit/Hooks/useActors';
import { useVerifyChain, useVerifyState } from '@/Modules/Audit/Hooks/useVerifyChain';
import { EMPTY_FILTER, isFiltered, isRangeValid, toQuery, type IAuditFilter } from '@/Modules/Audit/Types/AuditFilter';

const Forbidden = () => {
  const t = useT();
  return (
    <Card>
      <EmptyState icon="shield" text={t('audit.forbidden')} />
    </Card>
  );
};

export interface IAuditBodyProps {
  onOpenEvent?: (eventId: string) => void;
}

const AuditContent = ({ organizationId, onOpenEvent }: IAuditBodyProps & { organizationId: string }) => {
  const t = useT();
  const errorText = useApiErrorText();
  const [Page, setPage] = useState(1);
  const [Filter, setFilter] = useState<IAuditFilter>(EMPTY_FILTER);
  const [Applied, setApplied] = useState<IAuditFilter>(EMPTY_FILTER);
  const events = useAuditEvents(organizationId, Page, toQuery(Applied));
  const verify = useVerifyChain(organizationId);
  const verifyState = useVerifyState(organizationId);
  const actors = useActors(organizationId);
  const HandleFilter = (next: IAuditFilter) => {
    setFilter(next);
    if (!isRangeValid(next)) return;
    setApplied(next);
    setPage(1);
  };

  if (isApiError(events.error) && events.error.Status === 403) return <Forbidden />;
  if (events.isError) {
    return (
      <div className="grid justify-items-start gap-3">
        <InlineAlert accent="failed" title={t('audit.error.title')}>
          {errorText(events.error)}
        </InlineAlert>
        <Button
          size="sm"
          onClick={() => {
            void events.refetch();
          }}
        >
          {t('status.retry')}
        </Button>
      </div>
    );
  }

  const meta = events.data?.Meta;
  return (
    <>
      <VerifyCard
        state={verifyState}
        onRetry={() => {
          verify.mutate();
        }}
      />
      <AuditFilters
        filter={Filter}
        members={actors.Members.data?.Items ?? []}
        onChange={HandleFilter}
        onClear={() => {
          HandleFilter(EMPTY_FILTER);
        }}
      />
      {events.data === undefined ? <Skeleton label={t('audit.loading')} lines={6} /> : null}
      {events.data?.Items.length === 0 ? (
        <Card>
          <EmptyState icon="file" text={isFiltered(Applied) ? t('audit.filter.empty') : t('audit.empty')} />
        </Card>
      ) : null}
      {events.data && events.data.Items.length > 0 ? (
        <Card
          title={t('audit.list.title')}
          icon="file"
          end={t('audit.list.total', { count: meta?.Total ?? events.data.Items.length })}
        >
          <Rows label={t('audit.list.title')}>
            {events.data.Items.map((event) => (
              <AuditEventRow
                key={event.Id}
                event={event}
                actor={actors.NameOf(event)}
                onOpen={
                  onOpenEvent
                    ? () => {
                        onOpenEvent(event.Id);
                      }
                    : undefined
                }
              />
            ))}
          </Rows>
        </Card>
      ) : null}
      {meta ? (
        <Pager page={meta.Page} limit={meta.Limit || AUDIT_PAGE_SIZE} total={meta.Total} busy={events.isFetching} onPage={setPage} />
      ) : null}
    </>
  );
};

export const AuditBody = ({ onOpenEvent }: IAuditBodyProps) => {
  const membership = useCurrentMembership();
  if (membership === undefined || !canReadAudit(membership.Role)) return <Forbidden />;
  return <AuditContent organizationId={membership.OrganizationId} onOpenEvent={onOpenEvent} />;
};
