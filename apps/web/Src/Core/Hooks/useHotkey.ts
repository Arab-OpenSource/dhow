import { useEffect, useEffectEvent } from 'react';

export const useHotkey = (key: string, handler: () => void): void => {
  const onHotkey = useEffectEvent(handler);

  useEffect(() => {
    const listener = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === key.toLowerCase()) {
        event.preventDefault();
        onHotkey();
      }
    };
    document.addEventListener('keydown', listener);
    return () => {
      document.removeEventListener('keydown', listener);
    };
  }, [key]);
};
