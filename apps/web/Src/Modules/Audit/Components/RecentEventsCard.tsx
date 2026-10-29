import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { CardStates, cardStateOf } from '@/Core/Ui/CardStates';
import { Rows } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import { AuditEventRow } from '@/Modules/Audit/Components/AuditEventRow';
import { useActors } from '@/Modules/Audit/Hooks/useActors';
import { useRecentAuditEvents } from '@/Modules/Audit/Hooks/useAuditEvents';

export interface IRecentEventsCardProps {
  organizationId: string;
  onOpenEvent: (eventId: string) => void;
  onShowAll: () => void;
  className?: string;
}

export const RecentEventsCard = ({ organizationId, onOpenEvent, onShowAll, className }: IRecentEventsCardProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const events = useRecentAuditEvents(organizationId);
  const actors = useActors(organizationId);
  const items = events.data?.Items ?? [];

  return (
    <Card
      title={t('home.events.title')}
      icon="file"
      className={className}
      end={
        items.length > 0 ? (
          <Button size="sm" variant="ghost" onClick={onShowAll}>
            {t('home.events.all')}
          </Button>
        ) : null
      }
    >
      <CardStates
        state={cardStateOf(events, items.length === 0)}
        loadingLabel={t('home.events.loading')}
        errorTitle={t('home.events.error')}
        errorText={events.error ? errorText(events.error) : undefined}
        retryLabel={t('jobs.retry')}
        onRetry={() => {
          void events.refetch();
        }}
        emptyIcon="file"
        emptyText={t('home.events.empty')}
      >
        <Rows label={t('home.events.title')}>
          {items.map((event) => (
            <AuditEventRow
              key={event.Id}
              event={event}
              actor={actors.NameOf(event)}
              onOpen={() => {
                onOpenEvent(event.Id);
              }}
            />
          ))}
        </Rows>
      </CardStates>
    </Card>
  );
};
