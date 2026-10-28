import { useFactorMethods } from '@/Core/Auth/useMfaStatus';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { ConfirmPanel } from '@/Core/Ui/ConfirmPanel';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Row, Rows } from '@/Core/Ui/Row';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { useIdentitiesCard, type IIdentityEntry } from '@/Modules/Session/Hooks/useIdentities';

export interface IIdentitiesCardProps {
  navigate?: (url: string) => void;
}

const IdentityRow = ({ entry, onAsk }: { entry: IIdentityEntry; onAsk: () => void }) => {
  const t = useT();
  const time = useRelativeTime();
  const linked = entry.Identity;
  const used = linked?.LastUsedAt ? t('identities.lastUsed', { when: time.ago(linked.LastUsedAt) }) : null;
  const sub = linked === null ? t('identities.notLinked') : [linked.Email || t('identities.linked'), used].filter(Boolean).join(' · ');

  return (
    <Row
      icon="user"
      title={entry.Name}
      sub={sub}
      end={
        <>
          {linked !== null ? <Badge tone="ok">{t('identities.linked')}</Badge> : null}
          {!entry.Configured ? <Badge tone="mute">{t('identities.retired')}</Badge> : null}
          <Button size="sm" variant={linked === null ? 'secondary' : 'ghost'} iconStart={linked === null ? 'plus' : 'trash'} onClick={onAsk}>
            {linked === null ? t('identities.link') : t('identities.unlink')}
          </Button>
        </>
      }
    />
  );
};

export const IdentitiesCard = ({ navigate }: IIdentitiesCardProps) => {
  const t = useT();
  const methods = useFactorMethods();
  const card = useIdentitiesCard(methods ?? ['password'], navigate);

  if (card.Failed) return null;
  if (card.Entries === undefined || methods === undefined) return <Skeleton label={t('identities.loading')} lines={2} />;
  if (card.Entries.length === 0) return null;

  return (
    <Card title={t('identities.title')} icon="user">
      <p className="m-0 px-4 pb-2 text-body-sm text-secondary">{t('identities.lead')}</p>
      <Rows label={t('identities.title')}>
        {card.Entries.map((entry) => [
          <IdentityRow
            key={entry.Key}
            entry={entry}
            onAsk={() => {
              card.Ask(entry.Key);
            }}
          />,
          card.Open === entry.Key ? (
            <li key={`${entry.Key}-confirm`}>
              <ConfirmPanel
                question={entry.Identity === null ? t('identities.link.question', { provider: entry.Name }) : t('identities.unlink.question', { provider: entry.Name })}
                confirm={entry.Identity === null ? t('identities.link.confirm', { provider: entry.Name }) : t('identities.unlink.confirm')}
                icon={entry.Identity === null ? 'next' : 'trash'}
                danger={entry.Identity !== null}
                methods={methods}
                action={card.Action}
                onConfirm={card.Confirm}
                onCancel={card.Cancel}
              />
            </li>
          ) : null,
        ])}
      </Rows>
      {!entryConfiguredAny(card.Entries) ? <InlineAlert accent="awaiting" title={t('identities.noneConfigured')} className="mx-4 mb-4 w-auto" /> : null}
    </Card>
  );
};

const entryConfiguredAny = (entries: readonly IIdentityEntry[]): boolean => entries.some((entry) => entry.Configured);
