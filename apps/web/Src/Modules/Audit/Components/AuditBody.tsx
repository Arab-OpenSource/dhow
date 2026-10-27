import { useState } from 'react';

import { isApiError } from '@/Core/Api/ApiError';
import { canReadAudit } from '@/Core/Auth/AuthTypes';
import { useMembers } from '@/Core/Auth/useMembers';
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
import type { TAuditEvent } from '@/Modules/Audit/Api/AuditApi';
import { AuditEventRow } from '@/Modules/Audit/Components/AuditEventRow';
import { VerifyCard } from '@/Modules/Audit/Components/VerifyCard';
import { AUDIT_PAGE_SIZE, useAuditEvents } from '@/Modules/Audit/Hooks/useAuditEvents';
import { useVerifyChain, useVerifyState } from '@/Modules/Audit/Hooks/useVerifyChain';

const MEMBER_LOOKUP_LIMIT = 100;

const useActorName = (organizationId: string): ((event: TAuditEvent) => string) => {
  const t = useT();
  const members = useMembers(organizationId, 1, MEMBER_LOOKUP_LIMIT);
  const names = new Map((members.data?.Items ?? []).map((member) => [member.UserId, member.Name]));

  return (event) => {
    if (event.ActorUserId === undefined) {
      return event.Action === 'auth.login_failed' ? t('audit.actor.visitor') : t('audit.actor.system');
    }
    return names.get(event.ActorUserId) ?? t('audit.actor.unknown');
  };
};

const Forbidden = () => {
  const t = useT();
  return (
    <Card>
      <EmptyState icon="shield" text={t('audit.forbidden')} />
    </Card>
  );
};

const AuditContent = ({ organizationId }: { organizationId: string }) => {
  const t = useT();
  const errorText = useApiErrorText();
  const [Page, setPage] = useState(1);
  const events = useAuditEvents(organizationId, Page);
  const verify = useVerifyChain(organizationId);
  const verifyState = useVerifyState(organizationId);
  const actorName = useActorName(organizationId);

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
      {events.data === undefined ? <Skeleton label={t('audit.loading')} lines={6} /> : null}
      {events.data?.Items.length === 0 ? (
        <Card>
          <EmptyState icon="file" text={t('audit.empty')} />
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
              <AuditEventRow key={event.Id} event={event} actor={actorName(event)} />
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

export const AuditBody = () => {
  const membership = useCurrentMembership();
  if (membership === undefined || !canReadAudit(membership.Role)) return <Forbidden />;
  return <AuditContent organizationId={membership.OrganizationId} />;
};
