import { backendFailure, type TRouteHandler } from '@/Core/Testing/Backend';

export type TGateMode = 'hang' | 'fail' | 'pass';

export interface IGate {
  route: TRouteHandler;
  set: (mode: TGateMode) => void;
  hits: () => number;
}

export const gate = (pattern: RegExp, mode: TGateMode = 'pass'): IGate => {
  let current = mode;
  let count = 0;
  return {
    route: (request, url) => {
      if (request.method !== 'GET' || !pattern.test(url.pathname)) return undefined;
      count += 1;
      if (current === 'hang') return new Promise<Response>(() => undefined);
      if (current === 'fail') return backendFailure('INTERNAL', 'حدث خطأ في السيرفر.', 500);
      return undefined;
    },
    set: (next) => {
      current = next;
    },
    hits: () => count,
  };
};
