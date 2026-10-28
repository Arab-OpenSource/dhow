import { useNavigate } from '@tanstack/react-router';

import { useMe } from '@/Core/Auth/useMe';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { Row, Rows } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';

export const WorkspacePage = () => {
  const t = useT();
  const navigate = useNavigate();
  const me = useMe();
  const homeless = me.data?.Memberships.length === 0;

  const Open = (frame: 'system-status' | 'appearance') => {
    void navigate({ to: '/', search: (previous) => ({ ...previous, stack: [frame] }) });
  };

  return (
    <div className="grid max-w-[760px] gap-4">
      <h1 className="m-0 text-title-xl">{t('home.title')}</h1>
      {homeless ? (
        <Card title={t('organizations.none.title')} icon="building">
          <EmptyState icon="building" text={t('organizations.none.body')} />
        </Card>
      ) : null}
      <Card title={t('home.start.title')} icon="home">
        <Rows>
          <Row
            icon="shield"
            title={t('status.title')}
            sub={t('home.status.sub')}
            onSelect={() => {
              Open('system-status');
            }}
          />
          <Row
            icon="settings"
            title={t('appearance.title')}
            sub={t('home.appearance.sub')}
            onSelect={() => {
              Open('appearance');
            }}
          />
        </Rows>
      </Card>
      <Card title={t('home.projects.title')} icon="layers">
        <EmptyState icon="layers" text={t('home.projects.empty')} />
      </Card>
    </div>
  );
};
