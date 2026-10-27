import { Button } from '@/Core/Ui/Button';
import { Icon } from '@/Core/Ui/Icon';
import { useT } from '@/I18n/useT';

export interface IPagerProps {
  page: number;
  limit: number;
  total: number;
  busy?: boolean;
  onPage: (page: number) => void;
}

export const pageCount = (total: number, limit: number): number => Math.max(1, Math.ceil(total / Math.max(1, limit)));

export const Pager = ({ page, limit, total, busy = false, onPage }: IPagerProps) => {
  const t = useT();
  const pages = pageCount(total, limit);
  if (pages <= 1) return null;

  return (
    <nav aria-label={t('pager.label')} className="flex items-center justify-between gap-2">
      <Button
        size="sm"
        iconStart="back"
        disabled={busy || page <= 1}
        onClick={() => {
          onPage(page - 1);
        }}
      >
        {t('pager.previous')}
      </Button>
      <span className="text-caption text-secondary">{t('pager.status', { page, pages })}</span>
      <Button
        size="sm"
        disabled={busy || page >= pages}
        onClick={() => {
          onPage(page + 1);
        }}
      >
        {t('pager.next')}
        <Icon name="next" size={15} />
      </Button>
    </nav>
  );
};
