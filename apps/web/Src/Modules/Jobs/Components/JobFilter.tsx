import { cx } from '@/Core/Ui/ClassNames';
import { useT } from '@/I18n/useT';
import type { TJobState } from '@/Modules/Jobs/Api/JobsApi';
import { JOB_STATES } from '@/Modules/Jobs/Types/JobStates';

export interface IJobFilterProps {
  value: TJobState | null;
  disabled?: boolean;
  onChange: (state: TJobState | null) => void;
}

const CHOICES: readonly (TJobState | null)[] = [null, ...JOB_STATES];

export const JobFilter = ({ value, disabled = false, onChange }: IJobFilterProps) => {
  const t = useT();
  return (
    <div className="min-w-0">
      <div role="radiogroup" aria-label={t('jobs.filter.label')} className="flex gap-1.5 overflow-x-auto pb-1">
        {CHOICES.map((choice) => {
          const checked = choice === value;
          return (
            <button
              key={choice ?? 'all'}
              type="button"
              role="radio"
              aria-checked={checked}
              disabled={disabled}
              onClick={() => {
                onChange(choice);
              }}
              className={cx(
                'h-btn-sm flex-none cursor-pointer rounded-pill border px-3 text-label-sm whitespace-nowrap transition-colors duration-140',
                checked ? 'border-line-2 bg-surface-raised text-primary' : 'border-line bg-transparent text-secondary hover:text-primary',
              )}
            >
              {choice === null ? t('jobs.filter.all') : t(`jobs.state.${choice}`)}
            </button>
          );
        })}
      </div>
    </div>
  );
};
