import { Suspense } from 'react';

import { preloadAllFrames, preloadFrames } from '@/App/Layout/LazyFrames';
import type { TFrameId } from '@/App/Panels';
import type { ICommandPaletteProps } from '@/Core/Ui/CommandPalette';
import { lazyComponent } from '@/Core/Ui/LazyComponent';

const SlideHost = lazyComponent(() => import('@/App/Layout/SlideHost').then((module) => module.SlideHost));
const CommandPalette = lazyComponent(() => import('@/Core/Ui/CommandPalette').then((module) => module.CommandPalette));

export const preloadStack = (stack: readonly TFrameId[]): Promise<void> =>
  Promise.all([SlideHost.preload(), preloadFrames(stack)]).then(() => undefined);

export const preloadShellParts = (): void => {
  void SlideHost.preload();
  void CommandPalette.preload();
  void preloadAllFrames();
};

export const LazySlideHost = () => (
  <Suspense fallback={null}>
    <SlideHost />
  </Suspense>
);

export const LazyCommandPalette = (props: ICommandPaletteProps) => (
  <Suspense fallback={null}>
    <CommandPalette {...props} />
  </Suspense>
);
