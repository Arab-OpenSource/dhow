import { useId } from 'react';

const STOPS = [1, 2, 3, 4, 5, 6] as const;

export interface IMarkProps {
  size?: number;
  className?: string;
}

export const Mark = ({ size = 32, className }: IMarkProps) => {
  const gradientId = `sail-${useId().replaceAll(':', '')}`;
  return (
    <svg viewBox="0 0 32 32" width={size} height={size} aria-hidden="true" className={className}>
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="1" y2="1">
          {STOPS.map((stop, index) => (
            <stop key={stop} offset={index / (STOPS.length - 1)} style={{ stopColor: `var(--color-aurora-${stop})` }} />
          ))}
        </linearGradient>
      </defs>
      <path d="M17 3 L17 20.5 L5.5 20.5 Z" fill={`url(#${gradientId})`} />
      <path d="M18.6 7.5 L26.5 20.5 L18.6 20.5 Z" fill={`url(#${gradientId})`} opacity="0.55" />
      <path
        d="M3.5 23 H28.5 C27 26.6 23.8 28.5 20 28.5 H12 C8.2 28.5 5 26.6 3.5 23 Z"
        style={{ fill: 'var(--color-primary)' }}
      />
    </svg>
  );
};
