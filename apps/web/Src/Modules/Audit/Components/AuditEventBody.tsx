import type { ReactNode } from 'react';

import { isApiError } from '@/Core/Api/ApiError';
import { canReadAudit } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { Icon } from '@/Core/Ui/Icon';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { isMessageKey } from '@/I18n/Translate';
import { useT } from '@/I18n/useT';
import type { TAuditEvent } from '@/Modules/Audit/Api/AuditApi';
import { useActors } from '@/Modules/Audit/Hooks/useActors';
import { useAuditEvent } from '@/Modules/Audit/Hooks/useAuditEvents';
import { actionIcon, actionKey, isBadAction } from '@/Modules/Audit/Types/AuditActions';

const VIA_KEYS = new Set(['Via', 'ViaId']);

export interface IAuditEventBodyProps {
  eventId: string;
}

const Mono = ({ children }: { children: ReactNode }) => (
  <bdi dir="ltr" className="font-mono break-all">
    {children}
  </bdi>
);

const Fact = ({ label, children }: { label: string; children: ReactNode }) => (
  <div className="grid gap-0.5 border-t border-line px-4 py-3 first:border-t-0">
    <dt className="text-body-sm text-tertiary">{label}</dt>
    <dd className="m-0 min-w-0 text-body-sm text-primary">{children}</dd>
  </div>
);

const useTargetLabel = (): ((type: string) => string) => {
  const t = useT();
  return (type) => {
    const key = `audit.target.${type}`;
    return isMessageKey(key) ? t(key) : type;
  };
};

const EventDetail = ({ event, actor }: { event: TAuditEvent; actor: string }) => {
  const t = useT();
  const time = useRelativeTime();
  const targetLabel = useTargetLabel();
  const key = actionKey(event.Action);
  const data = Object.entries(event.Data).filter(([name]) => !VIA_KEYS.has(name));
  const viaToken = event.Data.Via === 'api_token';

  return (
    <>
      <Card padded>
        <div className="flex items-start gap-3">
          <Icon name={actionIcon(event.Action)} size={22} className="mt-0.5 text-secondary" />
          <div className="grid min-w-0 flex-1 gap-1">
            <h3 className="m-0 text-title-card">{key === null ? <Mono>{event.Action}</Mono> : t(key)}</h3>
            {key === null ? null : (
              <small className="text-caption text-tertiary">
                <Mono>{event.Action}</Mono>
              </small>
            )}
          </div>
          {isBadAction(event.Action) ? <Badge tone="bad">{t('audit.detail.attention')}</Badge> : null}
        </div>
      </Card>
      <Card title={t('audit.detail.facts')} icon="file">
        <dl className="m-0 grid">
          <Fact label={t('audit.detail.who')}>{actor}</Fact>
          <Fact label={t('audit.detail.when')}>
            <time dateTime={event.CreatedAt}>{time.full(event.CreatedAt)}</time>
            <span className="text-tertiary">{` · ${time.ago(event.CreatedAt)}`}</span>
          </Fact>
          {event.TargetType === '' ? null : (
            <Fact label={t('audit.detail.target')}>
              {targetLabel(event.TargetType)}
              {event.TargetId === '' ? null : (
                <span className="block text-caption text-secondary">
                  <Mono>{event.TargetId}</Mono>
                </span>
              )}
            </Fact>
          )}
          <Fact label={t('audit.detail.ip')}>{event.Ip === '' ? t('audit.detail.none') : <Mono>{event.Ip}</Mono>}</Fact>
          {viaToken ? (
            <Fact label={t('audit.detail.via')}>
              {t('audit.detail.via.token')}
              {event.Data.ViaId ? (
                <span className="block text-caption text-secondary">
                  <Mono>{event.Data.ViaId}</Mono>
                </span>
              ) : null}
            </Fact>
          ) : null}
          <Fact label={t('audit.detail.id')}>
            <Mono>{event.Id}</Mono>
          </Fact>
        </dl>
      </Card>
      <Card title={t('audit.detail.data')} icon="info">
        {data.length === 0 ? (
          <p className="m-0 px-4 pb-4 text-body-sm text-secondary">{t('audit.detail.data.empty')}</p>
        ) : (
          <dl className="m-0 grid">
            {data.map(([name, value]) => (
              <Fact key={name} label={name}>
                <Mono>{value}</Mono>
              </Fact>
            ))}
          </dl>
        )}
      </Card>
    </>
  );
};

const Loaded = ({ organizationId, eventId }: IAuditEventBodyProps & { organizationId: string }) => {
  const t = useT();
  const errorText = useApiErrorText();
  const event = useAuditEvent(organizationId, eventId);
  const actors = useActors(organizationId);

  if (isApiError(event.error) && event.error.Status === 404) {
    return (
      <Card>
        <EmptyState icon="file" text={t('audit.detail.missing')} />
      </Card>
    );
  }
  if (event.isError) {
    return (
      <div className="grid justify-items-start gap-3">
        <InlineAlert accent="failed" title={t('audit.detail.error')}>
          {errorText(event.error)}
        </InlineAlert>
        <Button
          size="sm"
          onClick={() => {
            void event.refetch();
          }}
        >
          {t('status.retry')}
        </Button>
      </div>
    );
  }
  if (event.data === undefined) return <Skeleton label={t('audit.detail.loading')} lines={6} />;
  return <EventDetail event={event.data} actor={actors.NameOf(event.data)} />;
};

export const AuditEventBody = ({ eventId }: IAuditEventBodyProps) => {
  const t = useT();
  const membership = useCurrentMembership();
  if (membership === undefined || !canReadAudit(membership.Role)) {
    return (
      <Card>
        <EmptyState icon="shield" text={t('audit.forbidden')} />
      </Card>
    );
  }
  return <Loaded key={membership.OrganizationId} organizationId={membership.OrganizationId} eventId={eventId} />;
};
