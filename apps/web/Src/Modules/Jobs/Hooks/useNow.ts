import { useEffect, useState } from 'react';

export const TICK_MS = 1000;

export const useNow = (active: boolean, every = TICK_MS): number => {
  const [Now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!active) return undefined;
    const timer = setInterval(() => {
      setNow(Date.now());
    }, every);
    return () => {
      clearInterval(timer);
    };
  }, [active, every]);

  return Now;
};
