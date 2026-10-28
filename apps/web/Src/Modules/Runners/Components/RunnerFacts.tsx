import type { ReactNode } from 'react';

import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Card } from '@/Core/Ui/Card';
import { useT } from '@/I18n/useT';
import type { TRunner } from '@/Modules/Runners/Api/RunnersApi';
import { LabelList } from '@/Modules/Runners/Components/LabelList';

interface IFactProps {
  term: string;
  children: ReactNode;
}

const Fact = ({ term, children }: IFactProps) => (
  <div className="grid gap-1 border-t border-line px-4 py-3 first:border-t-0 sm:grid-cols-[10rem_1fr] sm:gap-3">
    <dt className="text-body-sm text-secondary">{term}</dt>
    <dd className="m-0 min-w-0 text-body-sm">{children}</dd>
  </div>
);

const Ltr = ({ children }: { children: string }) => (
  <bdi dir="ltr" className="font-mono">
    {children}
  </bdi>
);

export const RunnerFacts = ({ runner }: { runner: TRunner }) => {
  const t = useT();
  const time = useRelativeTime();
  const seen = runner.LastSeenAt ? `${time.ago(runner.LastSeenAt)} · ${time.full(runner.LastSeenAt)}` : t('runners.neverSeen');

  return (
    <Card title={t('runners.facts.title')} icon="info">
      <dl className="m-0 grid">
        <Fact term={t('runners.facts.lastSeen')}>{seen}</Fact>
        <Fact term={t('runners.facts.labels')}>
          <LabelList labels={runner.Labels} empty={t('runners.labels.none')} label={t('runners.facts.labels')} />
        </Fact>
        <Fact term={t('runners.facts.capabilities')}>
          <LabelList labels={runner.CapabilityLabels} empty={t('runners.labels.none')} label={t('runners.facts.capabilities')} />
        </Fact>
        <Fact term={t('runners.facts.platform')}>
          <Ltr>{`${runner.Os}/${runner.Arch}`}</Ltr>
        </Fact>
        <Fact term={t('runners.facts.isolation')}>
          {runner.IsolationLevels.map((level) => t(`runners.isolation.${level}`)).join(' · ')}
        </Fact>
        <Fact term={t('runners.facts.capacity')}>{t('runners.facts.capacity.value', { count: runner.Capacity })}</Fact>
        <Fact term={t('runners.facts.version')}>
          <Ltr>{runner.Version}</Ltr>
        </Fact>
        <Fact term={t('runners.facts.key')}>
          {t('runners.facts.key.value', { version: runner.KeyVersion, when: time.full(runner.KeysRotatedAt) })}
        </Fact>
        <Fact term={t('runners.facts.created')}>{time.full(runner.CreatedAt)}</Fact>
        {runner.RevokedAt ? <Fact term={t('runners.facts.revoked')}>{time.full(runner.RevokedAt)}</Fact> : null}
      </dl>
    </Card>
  );
};
