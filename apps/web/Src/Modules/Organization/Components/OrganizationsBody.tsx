import { useCurrentMembership, useMe } from '@/Core/Auth/useMe';
import { useOrganizationStore } from '@/Core/Store/useOrganizationStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { Row, Rows } from '@/Core/Ui/Row';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';

export interface IOrganizationsBodyProps {
  onSwitched: () => void;
  onSettings: () => void;
  onCreate: () => void;
}

export const OrganizationsBody = ({ onSwitched, onSettings, onCreate }: IOrganizationsBodyProps) => {
  const t = useT();
  const me = useMe();
  const current = useCurrentMembership();
  const Set = useOrganizationStore((state) => state.Set);
  const Push = useToastStore((state) => state.Push);

  if (me.data === undefined) return <Skeleton label={t('ui.loading')} lines={3} />;
  const { Memberships, CanCreateOrganizations } = me.data;

  return (
    <>
      <Card title={t('organizations.title')} icon="building">
        {Memberships.length === 0 ? (
          <EmptyState icon="building" text={t('organizations.none.body')} />
        ) : (
          <Rows label={t('organizations.title')}>
            {Memberships.map((membership) => {
              const active = membership.OrganizationId === current?.OrganizationId;
              return (
                <Row
                  key={membership.OrganizationId}
                  icon="building"
                  title={membership.OrganizationName}
                  sub={
                    <>
                      <bdi dir="ltr" className="font-mono">
                        {membership.OrganizationSlug}
                      </bdi>
                      {` · ${t(`role.${membership.Role}`)}`}
                    </>
                  }
                  end={active ? <Badge tone="ok" icon="check">{t('organizations.current')}</Badge> : undefined}
                  onSelect={
                    active
                      ? undefined
                      : () => {
                          Set(membership.OrganizationId);
                          Push({
                            Id: `switched-${membership.OrganizationId}`,
                            Accent: 'live',
                            Title: t('organizations.switched', { name: membership.OrganizationName }),
                          });
                          onSwitched();
                        }
                  }
                />
              );
            })}
          </Rows>
        )}
      </Card>
      <div className="flex flex-wrap gap-2">
        {current ? (
          <Button size="sm" iconStart="settings" onClick={onSettings}>
            {t('organizations.settings', { name: current.OrganizationName })}
          </Button>
        ) : null}
        {CanCreateOrganizations ? (
          <Button size="sm" iconStart="plus" onClick={onCreate}>
            {t('organizations.new')}
          </Button>
        ) : null}
      </div>
    </>
  );
};
