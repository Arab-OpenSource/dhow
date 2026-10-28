import { Badge } from '@/Core/Ui/Badge';
import { Card } from '@/Core/Ui/Card';
import { useT } from '@/I18n/useT';
import type { TJobSpec } from '@/Modules/Jobs/Api/JobsApi';
import { Facts, type IFact } from '@/Modules/Jobs/Components/Facts';
import { durationOf, formatBytes } from '@/Modules/Jobs/Types/Duration';

const MILLIS_PER_CORE = 1000;
export const GVISOR = 'gVisor';

const Mono = ({ children }: { children: string }) => (
  <bdi dir="ltr" className="font-mono break-all">
    {children}
  </bdi>
);

const useResourceFacts = (spec: TJobSpec): IFact[] => {
  const t = useT();
  const bytes = (n: number) => {
    const value = formatBytes(n);
    return t(value.Key, value.Params);
  };
  const resources = spec.Resources ?? {};
  const facts: IFact[] = [];
  if (resources.CpuMillis !== undefined && resources.CpuMillis > 0) {
    facts.push({ id: 'cpu', label: t('jobs.spec.cpu'), value: t('jobs.spec.cpuValue', { n: resources.CpuMillis / MILLIS_PER_CORE }) });
  }
  if (resources.MemoryBytes !== undefined && resources.MemoryBytes > 0) {
    facts.push({ id: 'memory', label: t('jobs.spec.memory'), value: bytes(resources.MemoryBytes) });
  }
  if (resources.DiskBytes !== undefined && resources.DiskBytes > 0) {
    facts.push({ id: 'disk', label: t('jobs.spec.disk'), value: bytes(resources.DiskBytes) });
  }
  if (resources.Pids !== undefined && resources.Pids > 0) facts.push({ id: 'pids', label: t('jobs.spec.pids'), value: resources.Pids });
  if (spec.TimeoutSeconds !== undefined && spec.TimeoutSeconds > 0) {
    const value = durationOf(spec.TimeoutSeconds);
    facts.push({ id: 'timeout', label: t('jobs.spec.timeout'), value: t(value.Key, value.Params) });
  }
  return facts;
};

const useSpecFacts = (spec: TJobSpec): IFact[] => {
  const t = useT();
  const egress = spec.Network?.Egress ?? [];
  const labels = spec.Labels ?? [];
  const secrets = spec.Secrets ?? [];
  const facts: IFact[] = [
    { id: 'image', label: t('jobs.spec.image'), value: <Mono>{spec.Image}</Mono> },
    { id: 'isolation', label: t('jobs.spec.isolation'), value: t(`jobs.spec.isolation.${spec.Isolation ?? 'container'}`, { name: GVISOR }) },
    {
      id: 'labels',
      label: t('jobs.spec.labels'),
      value:
        labels.length === 0 ? (
          t('jobs.spec.noLabels')
        ) : (
          <span className="flex flex-wrap gap-1.5">
            {labels.map((label) => (
              <Badge key={label} tone="mute">
                <Mono>{label}</Mono>
              </Badge>
            ))}
          </span>
        ),
    },
    ...useResourceFacts(spec),
    {
      id: 'egress',
      label: t('jobs.spec.egress'),
      value:
        egress.length === 0 ? (
          t('jobs.spec.noEgress')
        ) : (
          <ul className="m-0 grid list-none gap-0.5 p-0">
            {egress.map((host) => (
              <li key={host}>
                <Mono>{host}</Mono>
              </li>
            ))}
          </ul>
        ),
    },
  ];
  if (secrets.length > 0) facts.push({ id: 'secrets', label: t('jobs.spec.secrets'), value: t('jobs.spec.secretsCount', { count: secrets.length }) });
  return facts;
};

export const JobSpecCard = ({ spec }: { spec: TJobSpec }) => {
  const t = useT();
  const facts = useSpecFacts(spec);
  return (
    <Card title={t('jobs.spec.title')} icon="settings" padded>
      <div className="grid gap-4">
        <Facts facts={facts} />
        <div className="grid gap-2">
          <h4 className="m-0 text-label-sm text-secondary">{t('jobs.spec.steps')}</h4>
          <ol className="m-0 grid list-none gap-2 p-0">
            {spec.Steps.map((step, index) => (
              <li key={`${String(index)}-${step.Name}`} className="grid gap-1 rounded-md border border-line px-3 py-2">
                <span className="flex flex-wrap items-center gap-2 text-label-sm">
                  <span className="text-secondary">{t('jobs.spec.step', { n: index + 1 })}</span>
                  <bdi>{step.Name}</bdi>
                  {step.Mutating ? (
                    <Badge tone="warn" icon="alert">
                      {t('jobs.spec.mutating')}
                    </Badge>
                  ) : null}
                </span>
                <pre dir="ltr" className="m-0 overflow-x-auto font-mono text-code whitespace-pre-wrap break-all text-secondary">
                  {step.Run}
                </pre>
              </li>
            ))}
          </ol>
        </div>
      </div>
    </Card>
  );
};
