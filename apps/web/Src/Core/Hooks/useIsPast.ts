import { useEffect, useState } from 'react';

const MAX_TIMER_MS = 2_147_483_647;

const parse = (iso: string | undefined): number => (iso === undefined ? Number.NaN : Date.parse(iso));

export const useIsPast = (iso: string | undefined): boolean => {
  const at = parse(iso);
  const [Start] = useState(() => ({ At: at, Past: at <= Date.now() }));
  const [Fired, setFired] = useState<number | null>(null);

  useEffect(() => {
    if (Number.isNaN(at)) return undefined;
    const wait = Math.max(0, at - Date.now());
    if (wait > MAX_TIMER_MS) return undefined;
    const timer = setTimeout(() => {
      setFired(at);
    }, wait);
    return () => {
      clearTimeout(timer);
    };
  }, [at]);

  return Fired === at || (Start.At === at && Start.Past);
};
