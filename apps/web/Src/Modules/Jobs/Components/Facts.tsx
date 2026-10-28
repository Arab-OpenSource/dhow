import type { ReactNode } from 'react';

export interface IFact {
  id: string;
  label: string;
  value: ReactNode;
}

export const Facts = ({ facts }: { facts: readonly IFact[] }) => (
  <dl className="m-0 grid grid-cols-[minmax(0,auto)_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-body-sm">
    {facts.map((fact) => (
      <div key={fact.id} className="contents">
        <dt className="text-secondary">{fact.label}</dt>
        <dd className="m-0 min-w-0 break-words">{fact.value}</dd>
      </div>
    ))}
  </dl>
);
