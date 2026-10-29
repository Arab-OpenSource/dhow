import type { IAuditQuery, TAuditCategory } from '@/Modules/Audit/Api/AuditApi';

export const CATEGORIES: readonly TAuditCategory[] = ['auth', 'identity', 'runner', 'job', 'system'];

export interface IAuditFilter {
  Category: TAuditCategory | '';
  ActorUserId: string;
  FromDate: string;
  ToDate: string;
}

export const EMPTY_FILTER: IAuditFilter = { Category: '', ActorUserId: '', FromDate: '', ToDate: '' };

const DATE = /^\d{4}-\d{2}-\d{2}$/;

const dayStart = (date: string, addDays = 0): string | undefined => {
  if (!DATE.test(date)) return undefined;
  const [year, month, day] = date.split('-').map(Number);
  if (year === undefined || month === undefined || day === undefined) return undefined;
  return new Date(year, month - 1, day + addDays).toISOString();
};

export const isCategory = (value: string): value is TAuditCategory => CATEGORIES.some((category) => category === value);

export const isFiltered = (filter: IAuditFilter): boolean =>
  filter.Category !== '' || filter.ActorUserId !== '' || filter.FromDate !== '' || filter.ToDate !== '';

export const isRangeValid = (filter: IAuditFilter): boolean =>
  filter.FromDate === '' || filter.ToDate === '' || filter.FromDate <= filter.ToDate;

export const toQuery = (filter: IAuditFilter): IAuditQuery => {
  const query: IAuditQuery = {};
  if (filter.Category !== '') query.Category = filter.Category;
  if (filter.ActorUserId !== '') query.ActorUserId = filter.ActorUserId;
  const from = dayStart(filter.FromDate);
  const to = dayStart(filter.ToDate, 1);
  if (from !== undefined) query.From = from;
  if (to !== undefined) query.To = to;
  return query;
};
