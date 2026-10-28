import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Row } from '@/Core/Ui/Row';
import { StatusDot } from '@/Core/Ui/StatusDot';
import { useT } from '@/I18n/useT';
import type { TRunner } from '@/Modules/Runners/Api/RunnersApi';
import { VIEW_DOT, VIEW_LABEL, viewOf } from '@/Modules/Runners/Types/RunnerView';

export interface IRunnerRowProps {
  runner: TRunner;
  onOpen: () => void;
}

export const RunnerRow = ({ runner, onOpen }: IRunnerRowProps) => {
  const t = useT();
  const time = useRelativeTime();
  const view = viewOf(runner);
  const seen = runner.LastSeenAt ? t('runners.lastSeen', { when: time.ago(runner.LastSeenAt) }) : t('runners.neverSeen');
  const labels = runner.Labels.length === 0 ? t('runners.labels.none') : <bdi dir="ltr">{runner.Labels.join(' · ')}</bdi>;

  return (
    <Row
      icon="server"
      title={runner.Name}
      sub={
        <>
          {labels}
          {` · ${seen}`}
        </>
      }
      end={<StatusDot status={VIEW_DOT[view]}>{t(VIEW_LABEL[view])}</StatusDot>}
      onSelect={onOpen}
    />
  );
};
