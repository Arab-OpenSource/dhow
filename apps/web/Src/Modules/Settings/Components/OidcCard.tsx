import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { CardStates, cardStateOf } from '@/Core/Ui/CardStates';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { Icon } from '@/Core/Ui/Icon';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';
import { OidcProviderRow } from '@/Modules/Settings/Components/OidcProviderRow';
import { useOidcSettings } from '@/Modules/Settings/Hooks/useOidcSettings';
import { PRODUCT_NAMES, type TOidcSettings } from '@/Modules/Settings/Types/OidcTypes';

export interface IOidcCardProps {
  onAdd: () => void;
  onEdit: (id: string) => void;
}

const Locked = ({ settings }: { settings: TOidcSettings }) => {
  const t = useT();
  return (
    <InlineAlert accent="building" title={t('settings.oidc.locked')}>
      <span className="grid gap-1.5">
        <span>{t('settings.oidc.lockedAbout')}</span>
        <span className="flex items-center gap-1.5">
          <Icon name="key" size={14} />
          <code dir="ltr" className="rounded-md border border-line bg-surface-inset px-1.5 font-mono text-code text-secondary">
            {settings.Env}
          </code>
        </span>
        {settings.IgnoredStored > 0 ? <span>{t('settings.oidc.ignored', { count: String(settings.IgnoredStored) })}</span> : null}
      </span>
    </InlineAlert>
  );
};

const Stranded = ({ count }: { count: number }) => {
  const t = useT();
  return (
    <InlineAlert accent="awaiting" title={t('settings.oidc.stranded')}>
      {t('settings.oidc.strandedAbout', { count: String(count) })}
    </InlineAlert>
  );
};

export const OidcCard = ({ onAdd, onEdit }: IOidcCardProps) => {
  const t = useT();
  const query = useOidcSettings();
  const settings = query.data;
  const full = settings !== undefined && settings.Providers.length >= settings.MaxProviders;
  const add = (
    <Button variant="primary" iconStart="plus" disabled={full} onClick={onAdd}>
      {t('settings.oidc.add')}
    </Button>
  );

  return (
    <Card title={t('settings.oidc.title')} icon="user">
      <CardStates
        state={cardStateOf(query, false)}
        loadingLabel={t('settings.oidc.loading')}
        errorTitle={t('settings.oidc.error')}
        retryLabel={t('settings.retry')}
        onRetry={() => {
          void query.refetch();
        }}
        emptyIcon="user"
        emptyText={t('settings.oidc.empty', PRODUCT_NAMES)}
        lines={3}
      >
        {settings === undefined ? null : (
          <>
            <div className="grid gap-4 px-card pt-1 pb-3">
              <p className="m-0 text-body-sm text-secondary">{t('settings.oidc.lead', PRODUCT_NAMES)}</p>
              {settings.Locked ? <Locked settings={settings} /> : null}
              {settings.Locked && settings.IgnoredLinkedIdentities > 0 ? <Stranded count={settings.IgnoredLinkedIdentities} /> : null}
            </div>
            {settings.Providers.length === 0 ? (
              <EmptyState icon="user" text={t('settings.oidc.empty', PRODUCT_NAMES)} action={settings.Locked ? undefined : add} className="pt-2" />
            ) : (
              <>
                <ul aria-label={t('settings.oidc.list')} className="m-0 grid list-none border-t border-line p-0">
                  {settings.Providers.map((provider) => (
                    <OidcProviderRow key={provider.Id} provider={provider} locked={settings.Locked} onEdit={onEdit} />
                  ))}
                </ul>
                {settings.Locked ? null : (
                  <div className="flex flex-wrap items-center gap-3 border-t border-line px-card py-card">
                    {add}
                    {full ? <span className="text-body-sm text-tertiary">{t('settings.oidc.limit', { max: String(settings.MaxProviders) })}</span> : null}
                  </div>
                )}
              </>
            )}
          </>
        )}
      </CardStates>
    </Card>
  );
};
