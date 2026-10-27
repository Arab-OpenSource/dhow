import type { ReactNode } from 'react';

import { Icon } from '@/Core/Ui/Icon';

export interface ITopBarProps {
  start?: ReactNode;
  searchLabel: string;
  shortcut: string;
  onSearch: () => void;
  children?: ReactNode;
}

export const TopBar = ({ start, searchLabel, shortcut, onSearch, children }: ITopBarProps) => (
  <header className="flex h-topbar flex-none items-center gap-6 bg-canvas px-4 md:px-6">
    {start}
    <button
      type="button"
      onClick={onSearch}
      aria-label={searchLabel}
      className={
        'edge flex h-[46px] max-w-[520px] flex-1 cursor-pointer items-center gap-2.5 rounded-pill px-[18px] ' +
        'text-start text-body-sm text-tertiary [--edge-fill:var(--color-input)] max-md:size-[46px] max-md:flex-none max-md:justify-center max-md:p-0'
      }
    >
      <Icon name="search" size={18} />
      <span className="max-md:hidden">{searchLabel}</span>
      <kbd className="ms-auto rounded-[6px] border border-default px-1.5 font-mono text-[11px] text-tertiary max-md:hidden">
        {shortcut}
      </kbd>
    </button>
    <div className="ms-auto flex items-center gap-3.5">{children}</div>
  </header>
);
