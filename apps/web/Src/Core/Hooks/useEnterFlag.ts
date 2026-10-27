import { useEffect, useState } from 'react';

export const useEnterFlag = (active: boolean, immediate: boolean): boolean => {
  const [Entered, setEntered] = useState(false);

  useEffect(() => {
    if (!active || immediate) return undefined;
    let second = 0;
    const first = requestAnimationFrame(() => {
      second = requestAnimationFrame(() => {
        setEntered(true);
      });
    });
    return () => {
      cancelAnimationFrame(first);
      cancelAnimationFrame(second);
      setEntered(false);
    };
  }, [active, immediate]);

  return active && (immediate || Entered);
};
