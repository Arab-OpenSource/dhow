import { useEffect } from 'react';

import { useCrumbStore } from '@/Core/Store/useCrumbStore';

export const useCrumbTitle = (id: string, title: string | undefined): void => {
  const publish = useCrumbStore((state) => state.Publish);
  const withdraw = useCrumbStore((state) => state.Withdraw);
  useEffect(() => {
    if (title === undefined || title === '') return undefined;
    publish(id, title);
    return () => {
      withdraw(id);
    };
  }, [id, title, publish, withdraw]);
};
