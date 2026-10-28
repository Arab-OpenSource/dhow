import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { StatusDot, type TStatus } from '@/Core/Ui/StatusDot';
import { useT } from '@/I18n/useT';
import { LogLine } from '@/Modules/Jobs/Components/LogLines';
import { useFollowScroll } from '@/Modules/Jobs/Hooks/useFollowScroll';
import { useJobLog, type TLogConnection } from '@/Modules/Jobs/Hooks/useJobLog';
import { MAX_LOG_LINES } from '@/Modules/Jobs/Types/LogBuffer';

export interface IJobLogProps {
  organizationId: string;
  jobId: string;
  attempt: number;
  stepNames: readonly string[];
  onEnded?: () => void;
}

const CONNECTION_STATUS: Record<TLogConnection, TStatus> = {
  connecting: 'queued',
  live: 'building',
  reconnecting: 'awaiting',
  ended: 'live',
  failed: 'failed',
};

const ConnectionDot = ({ connection }: { connection: TLogConnection }) => {
  const t = useT();
  return (
    <StatusDot status={CONNECTION_STATUS[connection]} className="justify-self-end">
      {t(`jobs.log.${connection}`)}
    </StatusDot>
  );
};

export const JobLog = ({ organizationId, jobId, attempt, stepNames, onEnded }: IJobLogProps) => {
  const t = useT();
  const log = useJobLog({ organizationId, jobId, attempt, onEnded });
  const { Ref: scrollRef, Following, HandleScroll, JumpToEnd } = useFollowScroll(log.Buffer);
  const lines = log.Buffer.Lines;
  const done = log.Connection === 'ended';

  return (
    <div className="grid gap-2.5">
      {log.Connection === 'failed' ? null : (
        <div role="status" className="flex items-center justify-end">
          <ConnectionDot connection={log.Connection} />
        </div>
      )}
      {log.Connection === 'failed' ? (
        <div className="grid justify-items-start gap-2">
          <InlineAlert accent="failed" title={t('jobs.log.failed')}>
            {t('jobs.log.failedAbout')}
          </InlineAlert>
          <Button size="sm" onClick={log.Retry}>
            {t('jobs.retry')}
          </Button>
        </div>
      ) : null}
      {log.Status?.Truncated ? <InlineAlert accent="awaiting" title={t('jobs.log.truncated')} /> : null}
      {log.Buffer.Dropped > 0 ? (
        <p className="m-0 text-caption text-tertiary">{t('jobs.log.dropped', { count: MAX_LOG_LINES })}</p>
      ) : null}
      <div
        ref={scrollRef}
        role="log"
        aria-label={t('jobs.log.label', { n: attempt })}
        /* eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex */
        tabIndex={0}
        onScroll={HandleScroll}
        className="max-h-[28rem] min-h-32 overflow-auto rounded-md border border-line bg-surface-inset px-3 py-2.5 font-mono text-code"
      >
        {lines.length === 0 ? (
          <p className="m-0 font-sans text-body-sm text-tertiary">{done ? t('jobs.log.empty') : t('jobs.log.waiting')}</p>
        ) : (
          lines.map((line) => <LogLine key={line.Id} line={line} stepNames={stepNames} />)
        )}
      </div>
      {!Following && lines.length > 0 ? (
        <Button size="sm" className="justify-self-start" onClick={JumpToEnd}>
          {t('jobs.log.jump')}
        </Button>
      ) : null}
    </div>
  );
};
