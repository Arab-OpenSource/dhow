import type { ReactNode } from 'react';

import { PageHeader } from '@/Core/Ui/PageHeader';

export interface IPageProps {
  title: string;
  lead?: ReactNode;
  action?: ReactNode;
  nav?: ReactNode;
  children: ReactNode;
}

export const Page = ({ title, lead, action, nav, children }: IPageProps) => (
  <div className="grid max-w-[1200px] gap-block">
    <PageHeader title={title} lead={lead} action={action} />
    {nav}
    <div className="grid gap-block pt-1">{children}</div>
  </div>
);
