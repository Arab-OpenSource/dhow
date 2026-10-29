import { scorePassword, type TStrength } from '@/Core/Auth/PasswordStrength';
import { cx } from '@/Core/Ui/ClassNames';
import type { TMessageKey } from '@/I18n/Messages';
import { useT } from '@/I18n/useT';

const SEGMENTS = [1, 2, 3, 4] as const;

const LABELS: Record<TStrength, TMessageKey> = {
  0: 'password.strength.empty',
  1: 'password.strength.weak',
  2: 'password.strength.ok',
  3: 'password.strength.good',
  4: 'password.strength.strong',
};

const FILLS: Record<TStrength, string> = {
  0: 'bg-track',
  1: 'bg-accent-coral',
  2: 'bg-accent-amber',
  3: 'bg-accent-green',
  4: 'bg-accent-green',
};

export interface IStrengthMeterProps {
  password: string;
}

export const StrengthMeter = ({ password }: IStrengthMeterProps) => {
  const t = useT();
  const score = scorePassword(password);
  const label = t(LABELS[score]);

  return (
    <div className="grid gap-1.5">
      <div
        role="meter"
        aria-label={t('password.strength.label')}
        aria-valuemin={0}
        aria-valuemax={4}
        aria-valuenow={score}
        aria-valuetext={label}
        data-score={score}
        className="flex gap-1.5"
      >
        {SEGMENTS.map((segment) => (
          <i
            key={segment}
            aria-hidden="true"
            className={cx('h-1 flex-1 rounded-pill', segment <= score ? FILLS[score] : 'bg-track')}
          />
        ))}
      </div>
      <p aria-live="polite" className="m-0 text-body-sm text-secondary">
        {label}
      </p>
    </div>
  );
};
