import { useEffect, useState } from 'react';

export const useDelayedFlag = (active: boolean, delayMs: number): boolean => {
  const [Elapsed, setElapsed] = useState(false);

  useEffect(() => {
    if (!active) return undefined;
    const timer = setTimeout(() => {
      setElapsed(true);
    }, delayMs);
    return () => {
      clearTimeout(timer);
      setElapsed(false);
    };
  }, [active, delayMs]);

  return active && Elapsed;
};
