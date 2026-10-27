import { useState } from 'react';

export const useLatch = (on: boolean): boolean => {
  const [Latched, setLatched] = useState(on);
  if (on && !Latched) setLatched(true);
  return Latched || on;
};
