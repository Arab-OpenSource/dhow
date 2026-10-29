import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import { followInApp } from '@/Core/Ui/PlainClick';
import { StatusDot } from '@/Core/Ui/StatusDot';
import { useT } from '@/I18n/useT';
import type { TRunner } from '@/Modules/Runners/Api/RunnersApi';
import { VIEW_DOT, VIEW_LABEL, viewOf } from '@/Modules/Runners/Types/RunnerView';

export interface IRunnerListRowProps {
  runner: TRunner;
  href: string;
  selected?: boolean;
  onOpen: () => void;
}

export const RunnerListRow = ({ runner, href, selected = false, onOpen }: IRunnerListRowProps) => {
  const t = useT();
  const time = useRelativeTime();
  const view = viewOf(runner);
  const seen = runner.LastSeenAt ? t('runners.lastSeen', { when: time.ago(runner.LastSeenAt) }) : t('runners.neverSeen');
  const labels = runner.Labels.length === 0 ? t('runners.labels.none') : <bdi dir="ltr">{runner.Labels.join(' · ')}</bdi>;

  return (
    <li className="min-w-0 border-t border-line first:border-t-0">
      <a
        href={href}
        aria-current={selected ? 'page' : undefined}
        onClick={followInApp(onOpen)}
        className={cx(
          'flex min-w-0 items-center gap-3 border-s-2 px-card py-3 text-primary no-underline transition-colors duration-140 [li:last-child_&]:rounded-b-md',
          selected ? 'border-accent bg-sel' : 'border-transparent hover:bg-surface-raised',
        )}
      >
        <Icon name="server" size={18} className={selected ? 'text-accent' : 'text-secondary'} />
        <span className="grid min-w-0 flex-1 gap-0.5">
          <span className="min-w-0 truncate text-label">
            <bdi>{runner.Name}</bdi>
          </span>
          <small className="min-w-0 truncate text-body-sm text-tertiary">
            {labels}
            {` · ${seen}`}
          </small>
        </span>
        <span className="flex flex-none items-center gap-1.5">
          <StatusDot status={VIEW_DOT[view]}>{t(VIEW_LABEL[view])}</StatusDot>
        </span>
        <Icon name="next" size={16} className="flex-none text-tertiary" />
      </a>
    </li>
  );
};
