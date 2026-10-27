import type { QueryClient } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';
import { useState } from 'react';

import { Providers } from '@/App/Providers';
import { createAppRouter } from '@/App/Router';
import { createQueryClient } from '@/Core/Api/QueryClient';

export interface IAppProps {
  router?: ReturnType<typeof createAppRouter>;
  queryClient?: QueryClient;
}

export const App = ({ router, queryClient }: IAppProps) => {
  const [Router] = useState(() => router ?? createAppRouter());
  const [Client] = useState(() => queryClient ?? createQueryClient());

  return (
    <Providers queryClient={Client}>
      <RouterProvider router={Router} />
    </Providers>
  );
};
