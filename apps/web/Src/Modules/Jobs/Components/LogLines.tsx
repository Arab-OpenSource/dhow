import { memo } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { useT } from '@/I18n/useT';
import { systemCodeOf, type ILogLine } from '@/Modules/Jobs/Types/LogBuffer';

const SYSTEM_CODES = ['LOG_TRUNCATED', 'LATE_RESULT'] as const;
type TSystemCode = (typeof SYSTEM_CODES)[number];

const isSystemCode = (code: string | null): code is TSystemCode => SYSTEM_CODES.some((known) => known === code);

const StepHeader = ({ line, stepNames }: { line: ILogLine; stepNames: readonly string[] }) => {
  const t = useT();
  const index = line.Step ?? 0;
  const name = stepNames[index];
  return (
    <div className="mt-2 border-t border-line pt-2 font-sans text-label-sm text-secondary first:mt-0 first:border-t-0 first:pt-0">
      {name === undefined ? t('jobs.log.stepHeaderUnnamed', { n: index + 1 }) : t('jobs.log.stepHeader', { n: index + 1, name })}
    </div>
  );
};

const SystemLine = ({ line }: { line: ILogLine }) => {
  const t = useT();
  const code = systemCodeOf(line.Raw);
  return (
    <div dir="auto" className="whitespace-pre-wrap break-words text-tertiary italic">
      {isSystemCode(code) ? t(`jobs.log.system.${code}`) : line.Raw}
    </div>
  );
};

export interface ILogLineProps {
  line: ILogLine;
  stepNames: readonly string[];
}

export const LogLine = memo(({ line, stepNames }: ILogLineProps) => {
  if (line.Kind === 'step') return <StepHeader line={line} stepNames={stepNames} />;
  if (line.Kind === 'system') return <SystemLine line={line} />;
  return (
    <div
      dir="auto"
      data-stream={line.Stream}
      className={cx('min-h-[1lh] whitespace-pre-wrap break-words', line.Stream === 'stderr' && 'border-s-2 border-bad-line ps-2')}
    >
      {line.Segments.map((segment, index) => (
        <span key={index} className={segment.ClassName === '' ? undefined : segment.ClassName}>
          {segment.Text}
        </span>
      ))}
    </div>
  );
});

LogLine.displayName = 'LogLine';
