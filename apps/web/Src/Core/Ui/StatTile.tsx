import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import type { TIconName } from '@/Core/Ui/Icons';
import { followInApp } from '@/Core/Ui/PlainClick';
import type { TStatusTone } from '@/Core/Ui/Tone';

export interface IStatTileProps {
  value: number;
  label: string;
  icon: TIconName;
  tone: TStatusTone;
  sub?: string;
  href: string;
  onSelect: () => void;
}

const TILE: Record<TStatusTone, string> = {
  ok: 'border-ok-line bg-ok-wash text-ok-fg',
  info: 'border-info-line bg-info-wash text-info-fg',
  queue: 'border-queue-line bg-queue-wash text-queue-fg',
  warn: 'border-warn-line bg-warn-wash text-warn-fg',
  bad: 'border-bad-line bg-bad-wash text-bad-fg',
};

export const StatTile = ({ value, label, icon, tone, sub, href, onSelect }: IStatTileProps) => (
  <a
    href={href}
    data-tone={tone}
    onClick={followInApp(onSelect)}
    className="grid min-w-0 content-between gap-4 rounded-md border border-line bg-surface p-card text-primary no-underline transition-colors duration-140 hover:border-line-2 hover:bg-surface-raised"
  >
    <span className="flex min-w-0 items-start justify-between gap-2">
      <span aria-hidden="true" className={cx('inline-grid size-9 flex-none place-items-center rounded-sm border', TILE[tone])}>
        <Icon name={icon} size={18} />
      </span>
      {sub ? <span className="min-w-0 truncate pt-2 text-caption text-tertiary">{sub}</span> : null}
    </span>
    <span className="grid min-w-0 gap-0.5">
      <b className="text-stat font-medium tabular-nums">{value}</b>
      <span className="text-body-sm text-secondary">{label}</span>
    </span>
  </a>
);
