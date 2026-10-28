export interface ILabelListProps {
  labels: readonly string[];
  empty: string;
  label: string;
}

export const LabelList = ({ labels, empty, label }: ILabelListProps) =>
  labels.length === 0 ? (
    <span className="text-body-sm text-tertiary">{empty}</span>
  ) : (
    <ul aria-label={label} className="m-0 flex list-none flex-wrap gap-1.5 p-0">
      {labels.map((item) => (
        <li key={item} className="rounded-sm border border-line px-2 py-0.5 text-caption text-secondary">
          <bdi dir="ltr" className="font-mono">
            {item}
          </bdi>
        </li>
      ))}
    </ul>
  );
