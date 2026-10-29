import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import type { TIconName } from '@/Core/Ui/Icons';
import { followInApp } from '@/Core/Ui/PlainClick';

export interface IBottomBarItem {
  id: string;
  icon: TIconName;
  label: string;
  href: string;
  current?: boolean;
  dot?: string;
  onSelect: () => void;
}

export interface IBottomBarProps {
  label: string;
  items: IBottomBarItem[];
}

export const BottomBar = ({ label, items }: IBottomBarProps) => (
  <nav
    aria-label={label}
    className="fixed inset-x-0 bottom-0 z-30 h-bottombar border-t border-line bg-rail px-1 md:hidden"
  >
    <ul className="m-0 flex h-full list-none items-stretch p-0">
      {items.map((item) => (
        <li key={item.id} className="flex min-w-0 flex-1">
          <a
            href={item.href}
            aria-current={item.current ? 'page' : undefined}
            onClick={followInApp(item.onSelect)}
            className={cx(
              'flex min-w-0 flex-1 flex-col items-center justify-center gap-1 rounded-sm text-caption no-underline transition-colors duration-140',
              item.current ? 'text-primary' : 'text-tertiary hover:text-primary',
            )}
          >
            <span className="relative inline-flex">
              <Icon name={item.icon} size={20} className={item.current ? 'text-accent' : undefined} />
              {item.dot === undefined ? null : (
                <i aria-hidden="true" data-dot="warn" className="absolute -end-1 -top-0.5 size-2 rounded-full bg-accent-amber ring-2 ring-rail" />
              )}
            </span>
            <span className="max-w-full truncate px-0.5">{item.label}</span>
            {item.dot === undefined ? null : <span className="sr-only">{item.dot}</span>}
          </a>
        </li>
      ))}
    </ul>
  </nav>
);
