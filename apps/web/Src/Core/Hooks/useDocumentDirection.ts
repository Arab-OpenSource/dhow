import { useSyncExternalStore } from 'react';

import type { TDirection } from '@/I18n/Locale';

const subscribe = (onChange: () => void): (() => void) => {
  const observer = new MutationObserver(onChange);
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['dir'] });
  return () => {
    observer.disconnect();
  };
};

const read = (): TDirection => (document.documentElement.dir === 'ltr' ? 'ltr' : 'rtl');

export const useDocumentDirection = (): TDirection => useSyncExternalStore(subscribe, read, () => 'rtl');
