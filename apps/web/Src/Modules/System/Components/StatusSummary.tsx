import { Summary } from '@/Core/Ui/Summary';
import { useT } from '@/I18n/useT';
import type { THealthStatus, TMeta } from '@/Modules/System/Types/SystemTypes';

export interface IStatusSummaryProps {
  meta: TMeta;
  health: THealthStatus;
}

export const StatusSummary = ({ meta, health }: IStatusSummaryProps) => {
  const t = useT();

  if (health.Checks.some((check) => !check.Ok)) {
    return <Summary tone="bad" icon="alert" title={t('status.summary.bad.title')} sub={t('status.summary.bad.sub')} />;
  }
  if (meta.SetupRequired) {
    return <Summary tone="warn" icon="alert" title={t('status.summary.setup.title')} sub={t('status.summary.setup.sub')} />;
  }
  return <Summary tone="ok" icon="shield" title={t('status.summary.ok.title')} sub={t('status.summary.ok.sub')} />;
};
