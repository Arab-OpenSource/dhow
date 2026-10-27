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
  <header className="flex h-topbar flex-none items-center gap-4 bg-canvas px-4 md:px-6">
    {start}
    <button
      type="button"
      onClick={onSearch}
      aria-label={searchLabel}
      className={
        'flex h-btn max-w-[480px] flex-1 cursor-pointer items-center gap-2.5 rounded-input border border-line-2 bg-input px-3.5 ' +
        'text-start text-body-sm text-tertiary hover:border-line-2 hover:text-secondary ' +
        'max-md:size-btn max-md:flex-none max-md:justify-center max-md:p-0'
      }
    >
      <Icon name="search" size={18} />
      <span className="max-md:hidden">{searchLabel}</span>
      <kbd className="ms-auto rounded-xs border border-line-2 px-1.5 font-mono text-caption text-tertiary max-md:hidden">
        {shortcut}
      </kbd>
    </button>
    <div className="ms-auto flex items-center gap-2">{children}</div>
  </header>
);
