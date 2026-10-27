import type { MouseEvent } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import type { TIconName } from '@/Core/Ui/Icons';
import { Mark } from '@/Core/Ui/Mark';
import { Tooltip } from '@/Core/Ui/Tooltip';

export interface IRailItem {
  id: string;
  icon: TIconName;
  label: string;
  href: string;
  current?: boolean;
  onSelect: () => void;
}

export interface IIconRailProps {
  label: string;
  items: IRailItem[];
  bottom?: IRailItem[];
}

const isPlainClick = (event: MouseEvent<HTMLAnchorElement>): boolean =>
  event.button === 0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey;

const RailLink = ({ item }: { item: IRailItem }) => (
  <Tooltip label={item.label} side="end">
    <a
      href={item.href}
      aria-label={item.label}
      aria-current={item.current ? 'page' : undefined}
      onClick={(event) => {
        if (isPlainClick(event)) {
          event.preventDefault();
          item.onSelect();
        }
      }}
      className={cx(
        'grid size-11 place-items-center rounded-full no-underline transition-colors duration-140',
        item.current ? 'bg-aurora text-on-light' : 'text-tertiary hover:bg-surface-raised hover:text-primary',
      )}
    >
      <Icon name={item.icon} size={20} />
    </a>
  </Tooltip>
);

export const IconRail = ({ label, items, bottom = [] }: IIconRailProps) => (
  <nav
    aria-label={label}
    className={cx(
      'flex items-center gap-1.5 bg-rail',
      'max-md:fixed max-md:inset-x-0 max-md:bottom-0 max-md:z-30 max-md:h-16 max-md:justify-around max-md:px-3',
      'md:w-rail md:flex-col md:py-[18px]',
    )}
  >
    <Mark className="mb-[22px] max-md:hidden" />
    {items.map((item) => (
      <RailLink key={item.id} item={item} />
    ))}
    <div className="grid gap-1.5 max-md:contents md:mt-auto">
      {bottom.map((item) => (
        <RailLink key={item.id} item={item} />
      ))}
    </div>
  </nav>
);
