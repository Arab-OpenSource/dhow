import { ACCENT_DOT, type TAccent } from '@/Core/Ui/Accent';
import { cx } from '@/Core/Ui/ClassNames';

export interface IToastProps {
  accent?: TAccent;
  title: string;
  detail?: string;
  dismissLabel: string;
  onDismiss: () => void;
}

export const Toast = ({ accent = 'live', title, detail, dismissLabel, onDismiss }: IToastProps) => (
  <div
    role="status"
    data-accent={accent}
    className={
      'flex min-w-[300px] items-center gap-3 rounded-lg bg-card-gradient px-4 py-3 text-[14px] ' +
      '[box-shadow:var(--shadow-card),var(--shadow-overlay)]'
    }
  >
    <i aria-hidden="true" className={cx('size-1.5 flex-none rounded-full', ACCENT_DOT[accent])} />
    <span className="grid flex-1">
      {title}
      {detail ? <span className="text-caption text-tertiary">{detail}</span> : null}
    </span>
    <button
      type="button"
      onClick={onDismiss}
      className="cursor-pointer border-0 bg-transparent text-caption text-tertiary hover:text-primary"
    >
      {dismissLabel}
    </button>
  </div>
);
