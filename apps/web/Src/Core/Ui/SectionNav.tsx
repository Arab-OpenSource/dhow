import { cx } from '@/Core/Ui/ClassNames';
import { followInApp } from '@/Core/Ui/PlainClick';

export interface ISectionNavItem {
  id: string;
  label: string;
  href: string;
  current?: boolean;
  onSelect: () => void;
}

export interface ISectionNavProps {
  label: string;
  items: ISectionNavItem[];
  className?: string;
}

export const SectionNav = ({ label, items, className }: ISectionNavProps) => (
  <nav aria-label={label} className={cx('min-w-0', className)}>
    <ul className="m-0 inline-flex max-w-full list-none gap-1 overflow-x-auto rounded-md border border-line bg-surface-inset p-1">
      {items.map((item) => (
        <li key={item.id} className="flex-none">
          <a
            href={item.href}
            aria-current={item.current ? 'page' : undefined}
            onClick={followInApp(item.onSelect)}
            className={cx(
              'flex h-btn-sm items-center rounded-sm px-3.5 text-label-sm whitespace-nowrap no-underline transition-colors duration-140',
              item.current ? 'bg-sel text-primary' : 'text-secondary hover:bg-hover hover:text-primary',
            )}
          >
            {item.label}
          </a>
        </li>
      ))}
    </ul>
  </nav>
);
