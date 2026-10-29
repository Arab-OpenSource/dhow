import { Suspense } from 'react';

import { preloadAllSlides, preloadSlide } from '@/App/Layout/LazySlides';
import type { TSlideId } from '@/App/Panels';
import type { ICommandPaletteProps } from '@/Core/Ui/CommandPalette';
import { lazyComponent } from '@/Core/Ui/LazyComponent';

const SlideHost = lazyComponent(() => import('@/App/Layout/SlideHost').then((module) => module.SlideHost));
const CommandPalette = lazyComponent(() => import('@/Core/Ui/CommandPalette').then((module) => module.CommandPalette));

export const preloadSlideParts = (slide: TSlideId): Promise<void> =>
  Promise.all([SlideHost.preload(), preloadSlide(slide)]).then(() => undefined);

export const preloadShellParts = (): void => {
  void SlideHost.preload();
  void CommandPalette.preload();
  void import('@/Core/Ui/MotionFeatures');
  void preloadAllSlides();
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
