import { useRef, type KeyboardEvent } from 'react';

import { cx } from '@/Core/Ui/ClassNames';

export interface ITab<T extends string> {
  id: T;
  label: string;
}

export interface ITabsProps<T extends string> {
  base: string;
  label: string;
  tabs: readonly ITab<T>[];
  value: T;
  onChange: (id: T) => void;
}

const step = (key: string, rtl: boolean): number => {
  if (key === 'ArrowLeft') return rtl ? 1 : -1;
  if (key === 'ArrowRight') return rtl ? -1 : 1;
  return 0;
};

export const tabPanelId = (base: string, id: string): string => `${base}-panel-${id}`;
export const tabId = (base: string, id: string): string => `${base}-tab-${id}`;

export const Tabs = <T extends string>(props: ITabsProps<T>) => {
  const { base, label, tabs, value, onChange } = props;
  const listRef = useRef<HTMLDivElement>(null);

  const HandleKey = (event: KeyboardEvent<HTMLButtonElement>) => {
    const rtl = getComputedStyle(event.currentTarget).direction === 'rtl';
    const at = tabs.findIndex((tab) => tab.id === value);
    let next = at + step(event.key, rtl);
    if (event.key === 'Home') next = 0;
    if (event.key === 'End') next = tabs.length - 1;
    if (next === at || next < 0 || next >= tabs.length) return;
    event.preventDefault();
    const target = tabs[next];
    if (target === undefined) return;
    onChange(target.id);
    listRef.current?.querySelector<HTMLElement>(`#${CSS.escape(tabId(base, target.id))}`)?.focus();
  };

  return (
    <div
      ref={listRef}
      role="tablist"
      aria-label={label}
      className="flex gap-1 overflow-x-auto rounded-md border border-line bg-surface-inset p-1"
    >
      {tabs.map((tab) => {
        const selected = tab.id === value;
        return (
          <button
            key={tab.id}
            id={tabId(base, tab.id)}
            type="button"
            role="tab"
            aria-selected={selected}
            aria-controls={tabPanelId(base, tab.id)}
            tabIndex={selected ? 0 : -1}
            onKeyDown={HandleKey}
            onClick={() => {
              onChange(tab.id);
            }}
            className={cx(
              'h-btn-sm flex-1 cursor-pointer rounded-btn border-0 px-3 text-label whitespace-nowrap transition-colors duration-140',
              selected ? 'bg-surface-raised text-primary' : 'bg-transparent text-secondary hover:text-primary',
            )}
          >
            {tab.label}
          </button>
        );
      })}
    </div>
  );
};
