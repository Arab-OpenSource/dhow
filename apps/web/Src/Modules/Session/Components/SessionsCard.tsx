import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Rows } from '@/Core/Ui/Row';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { RevokeOthersBar } from '@/Modules/Session/Components/RevokeOthersBar';
import { SessionRow } from '@/Modules/Session/Components/SessionRow';
import { ALL_OTHERS, useSessionActions } from '@/Modules/Session/Hooks/useSessionActions';
import { useSessions } from '@/Modules/Session/Hooks/useSessions';

export const SessionsCard = () => {
  const t = useT();
  const errorText = useApiErrorText();
  const sessions = useSessions();
  const actions = useSessionActions();
  const list = sessions.data ?? [];
  const others = list.filter((session) => !session.Current).length;

  if (sessions.isPending) return <Skeleton label={t('account.sessions.loading')} lines={4} />;
  if (sessions.isError) {
    return (
      <div className="grid justify-items-start gap-3">
        <InlineAlert accent="failed" title={t('account.sessions.error')}>
          {errorText(sessions.error)}
        </InlineAlert>
        <Button
          size="sm"
          onClick={() => {
            void sessions.refetch();
          }}
        >
          {t('status.retry')}
        </Button>
      </div>
    );
  }

  return (
    <>
      {actions.Notice ? <InlineAlert accent="failed" title={actions.Notice} /> : null}
      <Card
        title={t('account.sessions.title')}
        icon="monitor"
        end={list.length === 1 ? t('account.sessions.count.one') : t('account.sessions.count', { count: list.length })}
      >
        <Rows label={t('account.sessions.title')}>
          {list.map((session) => (
            <SessionRow
              key={session.Id}
              session={session}
              confirming={actions.Confirming === session.Id}
              busy={actions.Busy}
              onAsk={() => {
                actions.Ask(session.Id);
              }}
              onCancel={actions.Cancel}
              onConfirm={() => {
                actions.Revoke(session.Id);
              }}
            />
          ))}
        </Rows>
        <RevokeOthersBar
          others={others}
          confirming={actions.Confirming === ALL_OTHERS}
          busy={actions.Busy}
          onAsk={() => {
            actions.Ask(ALL_OTHERS);
          }}
          onCancel={actions.Cancel}
          onConfirm={actions.RevokeOthers}
        />
      </Card>
    </>
  );
};
