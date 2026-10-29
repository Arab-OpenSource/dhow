import { Icon } from '@/Core/Ui/Icon';
import { followInApp } from '@/Core/Ui/PlainClick';

export interface ICrumb {
  id: string;
  label: string;
  href?: string;
  onSelect?: () => void;
}

export interface IBreadcrumbsProps {
  label: string;
  crumbs: ICrumb[];
  className?: string;
}

const Crumb = ({ crumb, last }: { crumb: ICrumb; last: boolean }) => {
  if (last) {
    return (
      <span aria-current="page" className="truncate text-primary">
        <bdi>{crumb.label}</bdi>
      </span>
    );
  }
  if (crumb.href !== undefined && crumb.onSelect !== undefined) {
    return (
      <a
        href={crumb.href}
        onClick={followInApp(crumb.onSelect)}
        className="truncate rounded-xs text-secondary no-underline hover:text-primary"
      >
        <bdi>{crumb.label}</bdi>
      </a>
    );
  }
  return (
    <span className="truncate">
      <bdi>{crumb.label}</bdi>
    </span>
  );
};

export const Breadcrumbs = ({ label, crumbs, className }: IBreadcrumbsProps) => (
  <nav aria-label={label} className={className}>
    <ol className="m-0 flex min-w-0 list-none items-center gap-1.5 p-0 text-body-sm text-tertiary">
      {crumbs.map((crumb, index) => (
        <li key={crumb.id} className="flex min-w-0 items-center gap-1.5">
          {index > 0 ? <Icon name="next" size={14} className="flex-none text-tertiary" /> : null}
          <Crumb crumb={crumb} last={index === crumbs.length - 1} />
        </li>
      ))}
    </ol>
  </nav>
);
