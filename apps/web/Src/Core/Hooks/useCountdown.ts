import { useCallback, useEffect, useState } from 'react';

const TICK_MS = 1000;

export interface ICountdown {
  Remaining: number;
  Start: (seconds: number) => void;
}

export const useCountdown = (): ICountdown => {
  const [Deadline, setDeadline] = useState<number | null>(null);
  const [Now, setNow] = useState(0);

  const Remaining = Deadline === null ? 0 : Math.max(0, Math.ceil((Deadline - Now) / TICK_MS));
  const running = Remaining > 0;

  useEffect(() => {
    if (!running) return undefined;
    const timer = setInterval(() => {
      setNow(Date.now());
    }, TICK_MS);
    return () => {
      clearInterval(timer);
    };
  }, [running]);

  const Start = useCallback((seconds: number) => {
    const now = Date.now();
    setNow(now);
    setDeadline(now + seconds * TICK_MS);
  }, []);

  return { Remaining, Start };
};
