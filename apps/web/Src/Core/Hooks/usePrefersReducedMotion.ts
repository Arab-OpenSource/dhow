import { useSyncExternalStore } from 'react';

const QUERY = '(prefers-reduced-motion: reduce)';

const subscribe = (onChange: () => void): (() => void) => {
  const query = window.matchMedia(QUERY);
  query.addEventListener('change', onChange);
  return () => {
    query.removeEventListener('change', onChange);
  };
};

const read = (): boolean => window.matchMedia(QUERY).matches;

export const usePrefersReducedMotion = (): boolean => useSyncExternalStore(subscribe, read, () => false);
