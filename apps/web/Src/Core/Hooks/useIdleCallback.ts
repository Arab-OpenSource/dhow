import { useEffect, useEffectEvent } from 'react';

const IDLE_TIMEOUT_MS = 2000;
const FALLBACK_DELAY_MS = 300;

export const useIdleCallback = (run: () => void): void => {
  const onIdle = useEffectEvent(run);

  useEffect(() => {
    if ('requestIdleCallback' in window) {
      const handle = window.requestIdleCallback(
        () => {
          onIdle();
        },
        { timeout: IDLE_TIMEOUT_MS },
      );
      return () => {
        window.cancelIdleCallback(handle);
      };
    }
    const timer = setTimeout(() => {
      onIdle();
    }, FALLBACK_DELAY_MS);
    return () => {
      clearTimeout(timer);
    };
  }, []);
};
