import type { TMember } from '@/Core/Auth/useMembers';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { Field } from '@/Core/Ui/Field';
import { Input } from '@/Core/Ui/Input';
import { Select } from '@/Core/Ui/Select';
import { useT } from '@/I18n/useT';
import { CATEGORIES, isCategory, isFiltered, isRangeValid, type IAuditFilter } from '@/Modules/Audit/Types/AuditFilter';

export interface IAuditFiltersProps {
  filter: IAuditFilter;
  members: readonly TMember[];
  onChange: (filter: IAuditFilter) => void;
  onClear: () => void;
}

export const AuditFilters = ({ filter, members, onChange, onClear }: IAuditFiltersProps) => {
  const t = useT();
  const rangeError = isRangeValid(filter) ? undefined : t('audit.filter.range');

  return (
    <Card title={t('audit.filter.title')} icon="search" padded>
      <form
        role="search"
        aria-label={t('audit.filter.title')}
        className="grid gap-3 sm:grid-cols-2"
        onSubmit={(event) => {
          event.preventDefault();
        }}
      >
        <Field label={t('audit.filter.category')}>
          <Select
            compact
            value={filter.Category}
            options={[
              { value: '', label: t('audit.filter.category.all') },
              ...CATEGORIES.map((category) => ({ value: category, label: t(`audit.category.${category}`) })),
            ]}
            onChange={(event) => {
              const value = event.target.value;
              onChange({ ...filter, Category: isCategory(value) ? value : '' });
            }}
          />
        </Field>
        <Field label={t('audit.filter.actor')}>
          <Select
            compact
            value={filter.ActorUserId}
            options={[
              { value: '', label: t('audit.filter.actor.all') },
              ...members.map((member) => ({ value: member.UserId, label: member.Name })),
            ]}
            onChange={(event) => {
              onChange({ ...filter, ActorUserId: event.target.value });
            }}
          />
        </Field>
        <Field label={t('audit.filter.from')}>
          <Input
            compact
            type="date"
            dir="ltr"
            value={filter.FromDate}
            max={filter.ToDate === '' ? undefined : filter.ToDate}
            onChange={(event) => {
              onChange({ ...filter, FromDate: event.target.value });
            }}
          />
        </Field>
        <Field label={t('audit.filter.to')} error={rangeError}>
          <Input
            compact
            type="date"
            dir="ltr"
            value={filter.ToDate}
            min={filter.FromDate === '' ? undefined : filter.FromDate}
            onChange={(event) => {
              onChange({ ...filter, ToDate: event.target.value });
            }}
          />
        </Field>
        {isFiltered(filter) ? (
          <Button size="sm" variant="ghost" iconStart="close" className="justify-self-start sm:col-span-2" onClick={onClear}>
            {t('audit.filter.clear')}
          </Button>
        ) : null}
      </form>
    </Card>
  );
};
