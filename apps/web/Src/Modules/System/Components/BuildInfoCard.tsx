import type { ReactNode } from 'react';

import { Card } from '@/Core/Ui/Card';
import type { TMeta } from '@/Modules/System/Types/SystemTypes';
import { useT } from '@/I18n/useT';

const Row = ({ label, children }: { label: string; children: ReactNode }) => (
  <div className="flex items-center justify-between gap-3 rounded-md bg-surface-inset px-3 py-2.5 text-body-sm">
    <dt className="text-tertiary">{label}</dt>
    <dd className="m-0 text-primary">{children}</dd>
  </div>
);

const Mono = ({ children }: { children: string }) => (
  <bdi dir="ltr" className="font-mono text-[12.5px]">
    {children}
  </bdi>
);

export const BuildInfoCard = ({ meta }: { meta: TMeta }) => {
  const t = useT();

  return (
    <Card title={t('status.build.title')} icon="commit">
      <dl className="m-0 grid gap-1.5">
        <Row label={t('status.build.version')}>
          <Mono>{meta.Version}</Mono>
        </Row>
        <Row label={t('status.build.commit')}>
          <Mono>{meta.Commit}</Mono>
        </Row>
        <Row label={t('status.build.setup')}>
          {meta.SetupRequired ? t('status.build.setupPending') : t('status.build.setupDone')}
        </Row>
      </dl>
    </Card>
  );
};
