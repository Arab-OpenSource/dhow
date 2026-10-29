import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Row } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import type { TAuditEvent } from '@/Modules/Audit/Api/AuditApi';
import { actionIcon, actionKey, groupKey } from '@/Modules/Audit/Types/AuditActions';

export interface IAuditEventRowProps {
  event: TAuditEvent;
  actor: string;
  onOpen?: () => void;
}

export const AuditEventRow = ({ event, actor, onOpen }: IAuditEventRowProps) => {
  const t = useT();
  const time = useRelativeTime();
  const key = actionKey(event.Action);

  return (
    <Row
      onSelect={onOpen}
      icon={actionIcon(event.Action)}
      title={key === null ? t(groupKey(event.Action)) : t(key)}
      sub={
        <>
          {key === null ? (
            <>
              <bdi dir="ltr" className="font-mono">
                {event.Action}
              </bdi>
              {' · '}
            </>
          ) : null}
          <bdi>{actor}</bdi>
          {event.Ip === '' ? null : (
            <>
              {' · '}
              <bdi dir="ltr" className="font-mono">
                {event.Ip}
              </bdi>
            </>
          )}
          {' · '}
          <time dateTime={event.CreatedAt} title={time.full(event.CreatedAt)}>
            {time.ago(event.CreatedAt)}
          </time>
        </>
      }
    />
  );
};
