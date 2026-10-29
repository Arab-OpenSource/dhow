import { useNavigate } from '@tanstack/react-router';

import { GROUP_LABELS, sectionById, type ISection, type ISectionMatch } from '@/App/Navigation/Sections';
import { useSection } from '@/App/Navigation/useSection';
import { Breadcrumbs, type ICrumb } from '@/Core/Ui/Breadcrumbs';
import type { TTranslate } from '@/I18n/I18nContext';
import { useT } from '@/I18n/useT';
import { isCheckName } from '@/Modules/System/Types/SystemTypes';

const CHECK_PREFIX = 'checks/';

const detailLabel = ({ section, rest }: ISectionMatch, t: TTranslate): string | null => {
  if (rest === null) return null;
  if (section.id === 'system') {
    const name = rest.startsWith(CHECK_PREFIX) ? rest.slice(CHECK_PREFIX.length) : '';
    return isCheckName(name) ? t(`status.check.${name}`) : null;
  }
  return section.detail ? t(section.detail) : null;
};

export const crumbsFor = (match: ISectionMatch, t: TTranslate, open: (section: ISection) => void): ICrumb[] => {
  const { section } = match;
  const link = (entry: ISection): ICrumb => ({
    id: entry.id,
    label: t(entry.label),
    href: entry.path,
    onSelect: () => {
      open(entry);
    },
  });
  const crumbs: ICrumb[] = [];
  if (section.group) crumbs.push({ id: `group-${section.group}`, label: t(GROUP_LABELS[section.group]) });
  if (section.parent) crumbs.push(link(sectionById(section.parent)));
  crumbs.push(link(section));
  const detail = detailLabel(match, t);
  if (detail !== null) crumbs.push({ id: 'detail', label: detail });
  return crumbs;
};

export const ShellCrumbs = ({ className }: { className?: string }) => {
  const t = useT();
  const navigate = useNavigate();
  const match = useSection();
  if (match === null) return null;

  const crumbs = crumbsFor(match, t, (section) => {
    void navigate({ to: section.path });
  });
  return <Breadcrumbs label={t('shell.crumbs.label')} crumbs={crumbs} className={className} />;
};
