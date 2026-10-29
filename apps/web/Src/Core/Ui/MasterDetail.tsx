import type { ReactNode } from 'react';

export interface IMasterDetailProps {
  list: ReactNode;
  detail?: ReactNode;
  listLabel: string;
  detailLabel?: string;
}

export const MasterDetail = ({ list, detail, listLabel, detailLabel }: IMasterDetailProps) => {
  if (detail === undefined || detail === null) {
    return (
      <section aria-label={listLabel} className="grid min-w-0 gap-block">
        {list}
      </section>
    );
  }
  return (
    <div className="grid min-w-0 gap-block xl:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] xl:items-start">
      <section aria-label={listLabel} data-master-list="" className="grid min-w-0 gap-block max-xl:hidden">
        {list}
      </section>
      <section aria-label={detailLabel ?? listLabel} data-master-detail="" className="grid min-w-0 gap-block">
        {detail}
      </section>
    </div>
  );
};
