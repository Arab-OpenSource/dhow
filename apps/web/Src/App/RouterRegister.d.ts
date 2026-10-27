import type { createAppRouter } from '@/App/Router';

declare module '@tanstack/react-router' {
  // eslint-disable-next-line @typescript-eslint/naming-convention
  interface Register {
    router: ReturnType<typeof createAppRouter>;
  }
}
