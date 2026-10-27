import { Badge } from '@/Core/Ui/Badge';
import { Card } from '@/Core/Ui/Card';
import { Row, Rows } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import type { TMeta } from '@/Modules/System/Types/SystemTypes';

const Mono = ({ children }: { children: string }) => (
  <bdi dir="ltr" className="font-mono text-code text-primary">
    {children}
  </bdi>
);

export const BuildInfoCard = ({ meta }: { meta: TMeta }) => {
  const t = useT();

  return (
    <Card title={t('status.build.title')} icon="info">
      <Rows>
        <Row title={t('status.build.version')} end={<Mono>{meta.Version}</Mono>} />
        <Row title={t('status.build.commit')} end={<Mono>{meta.Commit}</Mono>} />
        <Row
          title={t('status.build.setup')}
          end={
            meta.SetupRequired ? (
              <Badge tone="warn" icon="alert">
                {t('status.build.setupPending')}
              </Badge>
            ) : (
              <Badge tone="ok" icon="check">
                {t('status.build.setupDone')}
              </Badge>
            )
          }
        />
      </Rows>
    </Card>
  );
};
