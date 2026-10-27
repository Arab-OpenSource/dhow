import { useSyncExternalStore } from 'react';

export type TDirection = 'rtl' | 'ltr';

const subscribe = (onChange: () => void): (() => void) => {
  const observer = new MutationObserver(onChange);
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['dir'] });
  return () => {
    observer.disconnect();
  };
};

const read = (): TDirection => (document.documentElement.dir === 'ltr' ? 'ltr' : 'rtl');

export const useDocumentDirection = (): TDirection => useSyncExternalStore(subscribe, read, () => 'rtl');
